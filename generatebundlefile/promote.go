package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type SDKClients struct {
	ecrClient              *ecrClient
	ecrPublicClient        *ecrPublicClient
	stsClient              *stsClient
	ecrClientRelease       *ecrClient
	ecrPublicClientRelease *ecrPublicClient
	stsClientRelease       *stsClient
	helmDriver             *helmDriver
}

// GetSDKClients is used to handle the creation of different SDK clients.
func GetSDKClients() (*SDKClients, error) {
	clients := &SDKClients{}
	var err error

	// STS Connection with us-west-2 region
	// This override ensures we don't create source clients for staging or prod accounts
	os.Setenv("AWS_PROFILE", "default")
	conf, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(ecrRegion))
	if err != nil {
		return nil, fmt.Errorf("loading default AWS config: %w", err)
	}
	stsclient := sts.NewFromConfig(conf)
	clients.stsClient, err = NewStsClient(stsclient, true)
	if err != nil {
		return nil, fmt.Errorf("Unable to create SDK connection to STS %s", err)
	}

	ecrClient := ecr.NewFromConfig(conf)
	clients.ecrClient, err = NewECRClient(ecrClient, true)
	if err != nil {
		return nil, fmt.Errorf("Unable to create SDK connection to ECR %s", err)
	}

	clients.stsClientRelease = clients.stsClient
	clients.ecrClientRelease = clients.ecrClient

	// ECR Public Connection with us-east-1 region
	conf, err = config.LoadDefaultConfig(context.TODO(), config.WithRegion(ecrPublicRegion))
	if err != nil {
		return nil, fmt.Errorf("loading default AWS config: %w", err)
	}
	client := ecrpublic.NewFromConfig(conf)
	clients.ecrPublicClient, err = NewECRPublicClient(client, true)
	if err != nil {
		return nil, fmt.Errorf("creating default public ECR client: %w", err)
	}

	return clients, nil
}

func (c *SDKClients) GetProfileSDKConnection(service, profile, region string) (*SDKClients, error) {
	if service == "" || profile == "" {
		return nil, fmt.Errorf("empty service or profile passed to GetProfileSDKConnection")
	}
	confWithProfile, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithSharedConfigProfile(profile))
	if err != nil {
		return nil, fmt.Errorf("creating public AWS client config: %w", err)
	}

	switch service {
	case "ecrpublic":
		clientWithProfile := ecrpublic.NewFromConfig(confWithProfile)
		c.ecrPublicClientRelease, err = NewECRPublicClient(clientWithProfile, true)
		if err != nil {
			return nil, fmt.Errorf("Unable to create SDK connection to ECR Public using another profile %s", err)
		}
		return c, nil
	case "ecr":
		clientWithProfile := ecr.NewFromConfig(confWithProfile)
		c.ecrClientRelease, err = NewECRClient(clientWithProfile, true)
		if err != nil {
			return nil, fmt.Errorf("Unable to create SDK connection to ECR using another profile %s", err)
		}
		return c, nil
	case "sts":
		clientWithProfile := sts.NewFromConfig(confWithProfile)
		c.stsClientRelease, err = NewStsClient(clientWithProfile, true)
		if err != nil {
			return nil, fmt.Errorf("Unable to create SDK connection to STS using another profile %s", err)
		}
		return c, nil
	}
	return nil, fmt.Errorf("gave service not supported by GetProfileSDKConnection(), consider adding it to the switch case")
}

// PromoteHelmChart will take a given repository, and authFile and handle helm and image promotion for the mentioned chart.
func (c *SDKClients) PromoteHelmChart(repository, authFile, tag string, copyImages bool) error {
	var name, version, sha string
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Error getting pwd %s", err)
	}

	// If we are not moving artifacts to the Private ECR Packages artifact account, get information from public ECR instead.
	// The first scenario runs for flags
	// --private-profile
	// --promote.
	// The 2nd scenario runs for flags
	// --public-profile.
	if Profile != "default" {
		name, version, sha, err = c.getNameAndVersionPublic(repository, tag, c.stsClientRelease.AccountID)
		if err != nil {
			return fmt.Errorf("Error getting name and version from helm chart: %s", err)
		}
	} else {
		name, version, sha, err = c.getNameAndVersion(repository, tag, c.stsClient.AccountID)
		if err != nil {
			return fmt.Errorf("Error getting name and version from helm chart: %s", err)
		}
	}

	// Pull the Helm chart to Helm Cache
	BundleLog.Info("Found Helm Chart to read requires.yaml for image information", "Chart", fmt.Sprintf("%s:%s", name, version))
	semVer := strings.Replace(version, "_", "+", 1) // TODO use the Semvar library instead of this hack.
	chartPath, err := c.helmDriver.PullHelmChart(name, semVer)
	if err != nil {
		return fmt.Errorf("Error pulling helmchart %s", err)
	}
	// Get the correct Repo Name from the flag based on ECR repo name formatting
	// since we prepend the github org on some repo's, and not on others.
	chartName, helmname, err := splitECRName(name)
	if err != nil {
		return fmt.Errorf("Failed splitECRName %s", err)
	}
	// Untar the helm .tgz to pwd and name the folder to the helm chart Name
	dest := filepath.Join(pwd, chartName)
	err = UnTarHelmChart(chartPath, chartName, dest)
	if err != nil {
		return fmt.Errorf("failed pulling helm release %s", err)
	}
	// Check for requires.yaml in the unpacked helm chart
	helmDest := filepath.Join(pwd, chartName, helmname)
	defer os.RemoveAll(helmDest)
	f, err := hasRequires(helmDest)
	if err != nil {
		return fmt.Errorf("Helm chart doesn't have requires.yaml inside %s", err)
	}
	// Unpack requires.yaml into a GO struct
	helmRequiresImages, err := validateHelmRequires(f)
	if err != nil {
		return fmt.Errorf("Unable to parse requires.yaml file to Go Struct %s", err)
	}

	// Create a 2nd struct since the the helm chart is going to Public ECR while the images are going to Private ECR.
	helmRequires := &Requires{
		Spec: RequiresSpec{
			Images: []Image{
				{
					Repository: chartName,
					Tag:        version,
					Digest:     sha,
				},
			},
		},
	}
	// Loop through each image, and the helm chart itself and check for existence in ECR Public, skip if we find the SHA already exists in destination.
	// If we don't find the SHA in public, we lookup the tag from Private Dev account, and move to Private Artifact account.
	// This runs for flags
	// --private-profile
	// --promote
	// --copy-images
	if copyImages {
		accountID := c.activeAccountID()
		for _, image := range helmRequiresImages.Spec.Images {
			checkSha, checkTag, err := c.CheckDestinationECR(image, image.Tag)
			// This is going to run a copy if only 1 check passes because there are scenarios where the correct SHA exists, but the tag is not in sync.
			// Copy with the correct image SHA, but a new tag will just write a new tag to ECR so it's safe to run.
			if checkSha && checkTag {
				BundleLog.Info("Image Digest and Tag already exists in destination location, skipping image copy", "Destination", fmt.Sprintf("docker://%s.dkr.ecr.us-west-2.amazonaws.com/%s:%s@%s", accountID, image.Repository, image.Tag, image.Digest))
				continue
			} else {
				BundleLog.Info("Image Digest and Tag don't exist in destination location, copying images to destination", "Location", fmt.Sprintf("%s/%s@%s", image.Repository, image.Tag, image.Digest))
				// We have cases with tag mismatch where the SHA is accurate, but the tag in the destination repo is not synced, this will sync it.
				if image.Tag == "" {
					image.Tag, err = c.ecrClient.tagFromSha(image.Repository, image.Digest, tag)
				}
				if err != nil {
					BundleLog.Error(err, "Unable to find Tag from Digest")
				}
				err = c.moveImage(image, authFile)
				if err != nil {
					return err
				}

				continue
			}
		}
	}

	// If we have the profile for the artifact account present, we skip moving helm charts since they go to the public ECR only.
	// This will move the Helm chart from Private ECR to Public ECR if it doesn't exist. This goes to either dev or prod depending on the flags passed in.
	// This runs for flags
	// --public-profile
	// --promote.
	if c.ecrClientRelease == nil {
		for _, image := range helmRequires.Spec.Images {
			// Check if the Helm chart is going to Prod, or dev account.
			destinationRegistry := c.ecrPublicClient.SourceRegistry
			if c.ecrPublicClientRelease != nil {
				destinationRegistry = c.ecrPublicClientRelease.SourceRegistry
			}
			checkSha, checkTag, err := c.CheckDestinationECR(image, image.Tag)
			if checkSha && checkTag {
				BundleLog.Info("Image Digest and Tag already exist in destination location, skipping", "Destination", fmt.Sprintf("docker://%s/%s:%s@%s", destinationRegistry, image.Repository, image.Tag, image.Digest))
				continue
			} else {
				if c.ecrPublicClientRelease == nil {
					source := fmt.Sprintf("docker://%s.dkr.ecr.us-west-2.amazonaws.com/%s:%s", c.stsClient.AccountID, image.Repository, image.Tag)
					destination := fmt.Sprintf("docker://%s/%s:%s", destinationRegistry, image.Repository, image.Tag)
					BundleLog.Info("Copying Helm Digest and Tag to destination location", "Location", fmt.Sprintf("%s/%s:%s %s", c.ecrPublicClient.SourceRegistry, image.Repository, image.Tag, image.Digest))
					err = copyImage(BundleLog, authFile, source, destination)
					if err != nil {
						return fmt.Errorf("Unable to copy image from source to destination repo %s", err)
					}
				} else {
					source := fmt.Sprintf("docker://%s/%s:%s", c.ecrPublicClient.SourceRegistry, image.Repository, image.Tag)
					destination := fmt.Sprintf("docker://%s/%s:%s", destinationRegistry, image.Repository, image.Tag)
					BundleLog.Info("Copying Helm Digest and Tag to destination location", "Location", fmt.Sprintf("%s/%s:%s %s", c.ecrPublicClient.SourceRegistry, image.Repository, image.Tag, image.Digest))
					err = copyImage(BundleLog, authFile, source, destination)
					if err != nil {
						return fmt.Errorf("Unable to copy image from source to destination repo %s", err)
					}
				}
			}
		}
	}
	return nil
}

func (c *SDKClients) CheckDestinationECR(image Image, version string) (bool, bool, error) {
	var checkSha, checkTag bool
	var err error
	var check CheckECR

	// Change the source destination check depending on release to dev or prod.
	destination := c.ecrPublicClient
	if c.ecrPublicClientRelease != nil {
		destination = c.ecrPublicClientRelease
	}

	// We either check in private ECR or public ECR depending on what's passed in.
	if c.stsClientRelease != nil {
		check = c.ecrClientRelease
	} else {
		check = destination
	}

	check = c.ecrPublicClient
	if c.ecrPublicClientRelease != nil {
		check = c.ecrPublicClientRelease
	}

	checkSha, err = check.shaExistsInRepository(image.Repository, image.Digest)
	if err != nil {
		return false, false, err
	}
	checkTag, err = check.tagExistsInRepository(image.Repository, version)
	if err != nil {
		return false, false, err
	}

	return checkSha, checkTag, nil
}

func (c *SDKClients) activeAccountID() string {
	if c.stsClientRelease != nil && c.stsClient != nil {
		return c.stsClient.AccountID
	}
	if c.stsClientRelease != nil {
		return c.stsClientRelease.AccountID
	}
	if c.stsClient != nil {
		return c.stsClient.AccountID
	}
	return ""
}

func (c *SDKClients) moveImage(image Image, authFile string) error {
	var destinationPrefix string
	accountID := c.activeAccountID()
	BundleLog.Info("Moving container images to ECR")
	sourcePrefix := fmt.Sprintf("docker://%s.dkr.ecr.us-west-2.amazonaws.com/%s", accountID, image.Repository)
	if c.ecrClientRelease != nil {
		destinationPrefix = fmt.Sprintf("docker://%s.dkr.ecr.us-west-2.amazonaws.com/%s", c.stsClientRelease.AccountID, image.Repository)
	} else if c.ecrPublicClientRelease != nil {
		destinationPrefix = fmt.Sprintf("docker://%s/%s", c.ecrPublicClientRelease.SourceRegistry, image.Repository)
	} else {
		destinationPrefix = fmt.Sprintf("docker://%s/%s", c.ecrPublicClient.SourceRegistry, image.Repository)
	}

	err := copyImage(BundleLog, authFile, sourcePrefix+":"+image.Tag, destinationPrefix+":"+image.Tag)
	if err != nil {
		return fmt.Errorf("unable to copy image tag from source to destination repo: %w", err)
	}

	// when the image.Tag doesn't match image.Digest in the source ECR, we need to move the Digest directly
	err = copyImage(BundleLog, authFile, sourcePrefix+"@"+image.Digest, destinationPrefix+"@"+image.Digest)
	if err != nil {
		return fmt.Errorf("unable to copy image digest from source to destination repo: %w", err)
	}

	return nil
}
