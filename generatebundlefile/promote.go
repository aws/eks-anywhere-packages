package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

type SDKClients struct {
	ecrPublicClient        *ecrPublicClient
	stsClient              *stsClient
	ecrClient              *ecrClient
	ecrPublicClientRelease *ecrPublicClient
}

func NewAWSSessionProfile(profile, region string) (aws.Config, error) {
	if profile == "" || region == "" {
		return aws.Config{}, fmt.Errorf("Empty profile or region passed to function")
	}
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithSharedConfigProfile(profile),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("Creating session with profile")
	}
	return cfg, nil
}

// GetSDKClients is used to handle the creation of different SDK clients.
func GetSDKClients(profile string) (*SDKClients, error) {
	clients := &SDKClients{}
	var err error
	clients.ecrPublicClient, err = NewECRPublicClient(true, nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to create SDK connection to ECR Public %s", err)
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
		cfg, err := NewAWSSessionProfile(profile, ecrPublicRegion)
		if err != nil {
			return nil, fmt.Errorf("Unable to create SDK connection to session to another profile %s", err)
		}
		clients.ecrPublicClientRelease, err = NewECRPublicClient(true, &cfg)
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
	BundleLog.Info("Promoting chart and image version", name, version)
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
	fmt.Printf("helmRequires.Spec.Images=%v\n", helmRequires.Spec.Images)
	// Loop through each image, and the helm chart itself and check for existance in ECR Public, skip if we find the SHA already exists in destination.
	// If we don't find the SHA in public, we lookup the tag from Private, and copy from private to Public with the same tag.
	for _, images := range helmRequires.Spec.Images {
		check, err := destination.shaExistsInRepository(images.Repository, images.Digest)
		if err != nil {
			return fmt.Errorf("Unable to complete sha lookup this is due to an ECRPublic DescribeImages failure %s", err)
		}
		if check {
			BundleLog.Info("Image Digest already exists in destination location......skipping", images.Repository, images.Digest)
			continue
		} else {
			// If using a profile we just copy from source account to destination account
			if crossAccount {
				err := c.copyImagePubPubDifferentAcct(BundleLog, authFile, images)
				if err != nil {
					return fmt.Errorf("Unable to copy image from source to destination repo %s", err)
				}
				continue
			}
			err := copyImagePrivPubSameAcct(BundleLog, authFile, c.stsClient, c.ecrPublicClient, images)
			if err != nil {
				return fmt.Errorf("Unable to copy image from source to destination repo %s", err)
			}
		}
	}
	return nil
}
