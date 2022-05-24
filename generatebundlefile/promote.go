package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
)

type SDKClients struct {
	ecrPublicClient        *ecrPublicClient
	stsClient              *stsClient
	ecrClient              *ecrClient
	ecrPublicClientRelease *ecrPublicClient
}

// GetSDKClients is used to handle the creation of different SDK clients.
func GetSDKClients(profile string) (*SDKClients, error) {
	clients := &SDKClients{}
	var err error

	conf, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(ecrPublicRegion))
	if err != nil {
		return nil, fmt.Errorf("loading default AWS config: %w", err)
	}
	client := ecrpublic.NewFromConfig(conf)
	clients.ecrPublicClient, err = NewECRPublicClient(client, true)
	if err != nil {
		return nil, fmt.Errorf("creating default public ECR client: %w", err)
	}

	clients.stsClient, err = NewStsClient(true)
	if err != nil {
		return nil, fmt.Errorf("Unable to create SDK connection to STS %s", err)
	}
	clients.ecrClient, err = NewECRClient(true)
	if err != nil {
		return nil, fmt.Errorf("Unable to create SDK connection to ECR %s", err)
	}

	if profile != "" {
		confWithProfile, err := config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(ecrPublicRegion),
			config.WithSharedConfigProfile(profile))
		if err != nil {
			return nil, fmt.Errorf("creating public AWS client config: %w", err)
		}

		clientWithProfile := ecrpublic.NewFromConfig(confWithProfile)
		clients.ecrPublicClientRelease, err = NewECRPublicClient(clientWithProfile, true)
		if err != nil {
			return nil, fmt.Errorf("Unable to create SDK connection to ECR Public another profile %s", err)
		}
	}

	return clients, nil
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
	fmt.Printf("Promoting chart and images for version %s %s\n", name, version)
	semVer := strings.Replace(version, "_", "+", 1) // TODO use the Semvar library instead of this hack.
	chartPath, err := PullHelmChart(name, semVer, authFile)
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

	// Change the source destination check depending on release or not
	destination := c.ecrPublicClient
	if crossAccount {
		destination = c.ecrPublicClientRelease
	}
	// Loop through each image, and the helm chart itself and check for existance in ECR Public, skip if we find the SHA already exists in destination.
	// If we don't find the SHA in public, we lookup the tag from Private, and copy from private to Public with the same tag.
	for _, images := range helmRequires.Spec.Images {
		checkSha, err := destination.shaExistsInRepository(images.Repository, images.Digest)
		if err != nil {
			return fmt.Errorf("Unable to complete sha lookup this is due to an ECRPublic DescribeImages failure %s", err)
		}
		checkTag, err := destination.tagExistsInRepository(images.Repository, version)
		if err != nil {
			return fmt.Errorf("Unable to complete tag lookup this is due to an ECRPublic DescribeImages failure %s", err)
		}
		// This is going to run a copy if only 1 check passes because there are scenarios where the correct SHA exists, but the tag is not in sync.
		// Copy with the correct image SHA, but a new tag will just write a new tag to ECR so it's safe to run.
		if checkSha && checkTag {
			fmt.Printf("Image Digest, and Tag already exists in destination location......skipping %s %s\n", images.Repository, images.Digest)
			continue
		} else {
			// If using a profile we just copy from source account to destination account
			if crossAccount {
				fmt.Printf("Image Digest, and Tag dont exist in destination location......copying to %s/%s:%s %s\n", c.ecrPublicClientRelease.SourceRegistry, images.Repository, version, images.Digest)
				err := c.copyImagePubPubDifferentAcct(BundleLog, authFile, version, images)
				if err != nil {
					return fmt.Errorf("Unable to copy image from source to destination repo %s", err)
				}
				continue
			} else {
				fmt.Printf("Image Digest, and Tag dont exist in destination location......copying to %s/%s:%s %s\n", c.ecrPublicClient.SourceRegistry, images.Repository, version, images.Digest)
				// We have cases with tag mismatch where the SHA is accurate, but the tag in the destination repo is not synced, this will sync it.
				images.Tag, err = c.ecrClient.tagFromSha(images.Repository, images.Digest)
				if err != nil {
					BundleLog.Error(err, "Unable to find Tag from Digest")
				}
				err := copyImagePrivPubSameAcct(BundleLog, authFile, version, c.stsClient, c.ecrPublicClient, images)
				if err != nil {
					return fmt.Errorf("Unable to copy image from source to destination repo %s", err)
				}
				continue
			}
		}
	}
	return nil
}
