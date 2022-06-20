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
}

// GetSDKClients is used to handle the creation of different SDK clients.
func GetSDKClients() (*SDKClients, error) {
	clients := &SDKClients{}
	var err error
	// ECR Public Connection with us-east-1 region
	conf, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(ecrPublicRegion))
	if err != nil {
		return nil, fmt.Errorf("loading default AWS config: %w", err)
	}
	client := ecrpublic.NewFromConfig(conf)
	clients.ecrPublicClient, err = NewECRPublicClient(client, true)
	if err != nil {
		return nil, fmt.Errorf("creating default public ECR client: %w", err)
	}

	// STS Connection with us-west-2 region
	conf, err = config.LoadDefaultConfig(context.TODO(), config.WithRegion(ecrRegion))
	if err != nil {
		return nil, fmt.Errorf("loading default AWS config: %w", err)
	}
	stsclient := sts.NewFromConfig(conf)
	clients.stsClient, err = NewStsClient(stsclient, true)
	if err != nil {
		return nil, fmt.Errorf("Unable to create SDK connection to STS %s", err)
	}

	// ECR Private Connection with us-west-2 region
	conf, err = config.LoadDefaultConfig(context.TODO(), config.WithRegion(ecrRegion))
	if err != nil {
		return nil, fmt.Errorf("loading default AWS config: %w", err)
	}
	ecrClient := ecr.NewFromConfig(conf)
	clients.ecrClient, err = NewECRClient(ecrClient, true)
	if err != nil {
		return nil, fmt.Errorf("Unable to create SDK connection to ECR %s", err)
	}
	return clients, nil
}

func (c *SDKClients) GetProfileSDKConnection(service, profile string) (*SDKClients, error) {
	if service == "" || profile == "" {
		return nil, fmt.Errorf("empty service, or profile passed to GetProfileSDKConnection")
	}
	var region = ecrRegion
	if service == "ecrpublic" {
		region = ecrPublicRegion
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
func (c *SDKClients) PromoteHelmChart(repository, authFile string, crossAccount bool) error {
	var name, version, sha string
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Error getting pwd %s", err)
	}
	if crossAccount {
		name, version, sha, err = c.getNameAndVersionPublic(repository, c.stsClient.AccountID)
		if err != nil {
			return fmt.Errorf("Error getting name and version from helmchart %s", err)
		}
	} else {
		name, version, sha, err = c.getNameAndVersion(repository, c.stsClient.AccountID)
		if err != nil {
			return fmt.Errorf("Error getting name and version from helmchart %s", err)
		}
	}

	// Pull the Helm chart to Helm Cache
	BundleLog.Info("Found Helm Chart to read for information ", "Chart", fmt.Sprintf("%s:%s", name, version))
	semVer := strings.Replace(version, "_", "+", 1) // TODO use the Semvar library instead of this hack.
	driver, err := NewHelm(BundleLog, authFile)
	if err != nil {
		return fmt.Errorf("Error creating helm driver %s", err)
	}
	chartPath, err := driver.PullHelmChart(name, semVer)
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
	f, err := hasRequires(helmDest)
	if err != nil {
		return fmt.Errorf("Helm chart doesn't have requires.yaml inside %s", err)
	}
	// Unpack requires.yaml into a GO struct
	helmRequires, err := validateHelmRequires(f)
	if err != nil {
		return fmt.Errorf("Unable to parse requires.yaml file to Go Struct %s", err)
	}
	// Add the helm chart to the struct before looping through lookup/promote since we need it promoted too.
	helmRequires.Spec.Images = append(helmRequires.Spec.Images, Image{Repository: chartName, Tag: version, Digest: sha})

	// Loop through each image, and the helm chart itself and check for existance in ECR Public, skip if we find the SHA already exists in destination.
	// If we don't find the SHA in public, we lookup the tag from Private, and copy from private to Public with the same tag.

	for _, images := range helmRequires.Spec.Images {
		checkSha, checkTag, err := c.CheckDestinationECR(images, images.Tag)
		// This is going to run a copy if only 1 check passes because there are scenarios where the correct SHA exists, but the tag is not in sync.
		// Copy with the correct image SHA, but a new tag will just write a new tag to ECR so it's safe to run.
		if checkSha && checkTag {
			BundleLog.Info("Digest, and Tag already exists in destination location......skipping", "Destination:", fmt.Sprintf("%s:%s  %s", images.Repository, images.Tag, images.Digest))
			continue
		} else {
			// If using a profile we just copy from source account to destination account
			if crossAccount {
				BundleLog.Info("Image Digest, and Tag dont exist in destination location...... copy to", "Location", fmt.Sprintf("%s/%s:%s %s", c.ecrPublicClientRelease.SourceRegistry, images.Repository, images.Tag, images.Digest))
				source := fmt.Sprintf("docker://%s/%s:%s", c.ecrPublicClient.SourceRegistry, images.Repository, images.Tag)
				destination := fmt.Sprintf("docker://%s/%s:%s", c.ecrPublicClientRelease.SourceRegistry, images.Repository, images.Tag)
				err := copyImage(BundleLog, authFile, source, destination)
				if err != nil {
					return fmt.Errorf("Unable to copy image from source to destination repo %s", err)
				}
				continue
			} else {
				BundleLog.Info("Image Digest, and Tag dont exist in destination location...... copy to", "Location", fmt.Sprintf("%s/%s sha:%s", images.Repository, images.Tag, images.Digest))
				// We have cases with tag mismatch where the SHA is accurate, but the tag in the destination repo is not synced, this will sync it.
				if images.Tag == "" {
					images.Tag, err = c.ecrClient.tagFromSha(images.Repository, images.Digest)
				}
				if err != nil {
					BundleLog.Error(err, "Unable to find Tag from Digest")
				}
				source := fmt.Sprintf("docker://%s.dkr.ecr.us-west-2.amazonaws.com/%s:%s", c.stsClient.AccountID, images.Repository, images.Tag)
				// TODO Remove this if/else logic once we move away from public ECR 100% so we don't need both cases.
				var destination string
				if c.stsClientRelease != nil {
					BundleLog.Info("Moving images to private ECR in artifact account")
					destination = fmt.Sprintf("docker://%s.dkr.ecr.us-west-2.amazonaws.com/%s:%s", c.stsClientRelease.AccountID, images.Repository, images.Tag)
				} else {
					BundleLog.Info("Moving images to Public ECR in same account")
					destination = fmt.Sprintf("docker://%s/%s:%s", c.ecrPublicClient.SourceRegistry, images.Repository, images.Tag)
				}
				BundleLog.Info("Running copy OCI artifact command....")
				err := copyImage(BundleLog, authFile, source, destination)
				if err != nil {
					return fmt.Errorf("Unable to copy image from source to destination repo %s", err)
				}
				continue
			}
		}
	}
	return nil
}

func (c *SDKClients) CheckDestinationECR(images Image, version string) (bool, bool, error) {
	var checkSha, checkTag bool
	var err error
	var check CheckECR

	// Change the source destination check depending on release to another account or not
	destination := c.ecrPublicClient
	if c.ecrPublicClientRelease != nil {
		destination = c.ecrPublicClientRelease
	}

	// Release to ECR private in another account if we did an sts lookup for the other account ID
	if c.stsClientRelease != nil {
		check = c.ecrClientRelease
	} else {
		check = destination
	}
	checkSha, err = check.shaExistsInRepository(images.Repository, images.Digest)
	if err != nil {
		return false, false, err
	}
	checkTag, err = check.tagExistsInRepository(images.Repository, version)
	if err != nil {
		return false, false, err
	}
	return checkSha, checkTag, nil
}
