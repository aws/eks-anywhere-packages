package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
	ecrpublictypes "github.com/aws/aws-sdk-go-v2/service/ecrpublic/types"
	"github.com/pkg/errors"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

type ecrPublicClient struct {
	publicRegistryClient
	AuthConfig     string
	SourceRegistry string
}

type publicRegistryClient interface {
	DescribeImages(ctx context.Context, params *ecrpublic.DescribeImagesInput, optFns ...func(*ecrpublic.Options)) (*ecrpublic.DescribeImagesOutput, error)
	DescribeRegistries(ctx context.Context, params *ecrpublic.DescribeRegistriesInput, optFns ...func(*ecrpublic.Options)) (*ecrpublic.DescribeRegistriesOutput, error)
	GetAuthorizationToken(ctx context.Context, params *ecrpublic.GetAuthorizationTokenInput, optFns ...func(*ecrpublic.Options)) (*ecrpublic.GetAuthorizationTokenOutput, error)
}

// NewECRPublicClient Creates a new ECR Client Public client
func NewECRPublicClient(client publicRegistryClient, needsCreds bool) (*ecrPublicClient, error) {
	ecrPublicClient := &ecrPublicClient{
		publicRegistryClient: client,
	}
	if needsCreds {
		authorizationToken, err := ecrPublicClient.GetPublicAuthToken()
		if err != nil {
			return nil, err
		}
		ecrPublicClient.AuthConfig = authorizationToken
		return ecrPublicClient, nil
	}
	return ecrPublicClient, nil
}

// Describe returns a list of ECR describe results, with Pagination from DescribeImages SDK request
func (c *ecrPublicClient) DescribePublic(describeInput *ecrpublic.DescribeImagesInput) ([]ecrpublictypes.ImageDetail, error) {
	var images []ecrpublictypes.ImageDetail
	resp, err := c.DescribeImages(context.TODO(), describeInput)
	if err != nil {
		return nil, fmt.Errorf("error: Unable to complete DescribeImagesRequest to ECR public. %s", err)
	}
	images = append(images, resp.ImageDetails...)
	if resp.NextToken != nil {
		next := describeInput
		next.NextToken = resp.NextToken
		nextdetails, _ := c.DescribePublic(next)
		images = append(images, nextdetails...)
	}
	return images, nil
}

// GetShaForPublicInputs returns a list of an images version/sha for given inputs to lookup
func (c *ecrPublicClient) GetShaForPublicInputs(project Project) ([]api.SourceVersion, error) {
	sourceVersion := []api.SourceVersion{}
	for _, tag := range project.Versions {
		if !strings.HasSuffix(tag.Name, "latest") {
			var imagelookup []ecrpublictypes.ImageIdentifier
			imagelookup = append(imagelookup, ecrpublictypes.ImageIdentifier{ImageTag: &tag.Name})
			ImageDetails, err := c.DescribePublic(&ecrpublic.DescribeImagesInput{
				RepositoryName: aws.String(project.Repository),
				ImageIds:       imagelookup,
			})
			if err != nil {
				return nil, fmt.Errorf("error: Unable to complete DescribeImagesRequest to ECR public. %s", err)
			}
			for _, images := range ImageDetails {
				if *images.ImageManifestMediaType != "application/vnd.oci.image.manifest.v1+json" || len(images.ImageTags) == 0 {
					continue
				}
				if len(images.ImageTags) > 0 {
					v := &api.SourceVersion{Name: tag.Name, Digest: *images.ImageDigest}
					sourceVersion = append(sourceVersion, *v)
					continue
				}
			}
		}
		//
		if tag.Name == "latest" {
			ImageDetails, err := c.DescribePublic(&ecrpublic.DescribeImagesInput{
				RepositoryName: aws.String(project.Repository),
			})
			if err != nil {
				return nil, fmt.Errorf("error: Unable to complete DescribeImagesRequest to ECR public. %s", err)
			}
			var images []ImageDetailsBothECR
			for _, image := range ImageDetails {
				details, _ := createECRImageDetails(ImageDetailsECR{PublicImageDetails: image})
				images = append(images, details)
			}
			sha, err := getLastestImageSha(images)
			if err != nil {
				return nil, err
			}
			sourceVersion = append(sourceVersion, *sha)
			continue
		}
		//
		if strings.HasSuffix(tag.Name, "-latest") {
			regex := regexp.MustCompile(`-latest`)
			splitVersion := regex.Split(tag.Name, -1) //extract out the version without the latest
			ImageDetails, err := c.DescribePublic(&ecrpublic.DescribeImagesInput{
				RepositoryName: aws.String(project.Repository),
			})
			if err != nil {
				return nil, fmt.Errorf("error: Unable to complete DescribeImagesRequest to ECR public. %s", err)
			}

			var images []ImageDetailsBothECR
			for _, image := range ImageDetails {
				details, _ := createECRImageDetails(ImageDetailsECR{PublicImageDetails: image})
				images = append(images, details)
			}
			filteredImageDetails := ImageTagFilter(images, splitVersion[0])
			sha, err := getLastestImageSha(filteredImageDetails)
			if err != nil {
				return nil, err
			}
			sourceVersion = append(sourceVersion, *sha)
			continue
		}
	}
	sourceVersion = removeDuplicates(sourceVersion)
	return sourceVersion, nil
}

// shaExistsInRepositoryPublic checks if a given OCI artifact exists in a destination repo using the sha sum.
func (c *ecrPublicClient) shaExistsInRepository(repository, sha string) (bool, error) {
	if repository == "" || sha == "" {
		return false, fmt.Errorf("Emtpy repository, or sha passed to the function")
	}
	var imagelookup []ecrpublictypes.ImageIdentifier
	imagelookup = append(imagelookup, ecrpublictypes.ImageIdentifier{ImageDigest: &sha})
	ImageDetails, err := c.DescribePublic(&ecrpublic.DescribeImagesInput{
		RepositoryName: aws.String(repository),
		ImageIds:       imagelookup,
	})
	if err != nil {
		if strings.Contains(err.Error(), "does not exist within the repository") == true {
			return false, nil
		}
	}
	for _, detail := range ImageDetails {
		if detail.ImageDigest != nil && *detail.ImageDigest == sha {
			return true, nil
		}
	}
	return false, nil
}

// tagExistsInRepository checks if a given OCI artifact exists in a destination repo using the sha sum.
func (c *ecrPublicClient) tagExistsInRepository(repository, tag string) (bool, error) {
	if repository == "" || tag == "" {
		return false, fmt.Errorf("Emtpy repository, or tag passed to the function")
	}
	var imagelookup []ecrpublictypes.ImageIdentifier
	imagelookup = append(imagelookup, ecrpublictypes.ImageIdentifier{ImageTag: &tag})
	ImageDetails, err := c.DescribePublic(&ecrpublic.DescribeImagesInput{
		RepositoryName: aws.String(repository),
		ImageIds:       imagelookup,
	})
	if err != nil {
		if strings.Contains(err.Error(), "does not exist within the repository") == true {
			return false, nil
		}
	}
	for _, detail := range ImageDetails {
		for _, Imagetag := range detail.ImageTags {
			if tag == Imagetag {
				return true, nil
			}
		}
	}
	return false, nil
}

// GetRegistryURI gets the current account's AWS ECR Public registry URI
func (c *ecrPublicClient) GetRegistryURI() (string, error) {
	registries, err := c.DescribeRegistries(context.TODO(), (&ecrpublic.DescribeRegistriesInput{}))
	if err != nil {
		return "", err
	}
	if len(registries.Registries) > 0 && registries.Registries[0].RegistryUri != nil && *registries.Registries[0].RegistryUri != "" {
		return *registries.Registries[0].RegistryUri, nil
	}
	return "", fmt.Errorf("Emtpy list of registries for the account")
}

// GetPublicAuthToken gets an authorization token from ECR public
func (c *ecrPublicClient) GetPublicAuthToken() (string, error) {
	authTokenOutput, err := c.GetAuthorizationToken(context.TODO(), &ecrpublic.GetAuthorizationTokenInput{})
	if err != nil {
		return "", errors.Cause(err)
	}
	authToken := *authTokenOutput.AuthorizationData.AuthorizationToken

	return authToken, nil
}

// getNameAndVersionPublic looks up the latest pushed helm chart's tag from a given repo name Full name in Public ECR OCI format.
func (c *SDKClients) getNameAndVersionPublic(repoName, tag, registryURI string) (string, string, string, error) {
	var version string
	var sha string
	splitname := strings.Split(repoName, ":") // TODO add a regex filter
	name := splitname[0]
	ecrname := fmt.Sprintf("%s/%s", c.ecrPublicClient.SourceRegistry, name)
	if len(splitname) > 0 {
		if !strings.HasSuffix(tag, "latest") {
			imageIDs := []ecrpublictypes.ImageIdentifier{{ImageTag: &tag}}
			ImageDetails, err := c.ecrPublicClient.DescribePublic(&ecrpublic.DescribeImagesInput{
				RepositoryName: aws.String(repoName),
				ImageIds:       imageIDs,
			})
			if err != nil {
				return "", "", "", fmt.Errorf("DescribeImagesRequest to public ECR failed: %w", err)
			}
			if len(ImageDetails) == 1 {
				return ecrname, tag, *ImageDetails[0].ImageDigest, nil
			}
		}
		ImageDetails, err := c.ecrPublicClient.DescribePublic(&ecrpublic.DescribeImagesInput{
			RepositoryName: aws.String(repoName),
		})
		if err != nil {
			return "", "", "", err
		}
		var images []ImageDetailsBothECR
		for _, image := range ImageDetails {
			details, err := createECRImageDetails(ImageDetailsECR{PublicImageDetails: image})
			if err != nil {
				return "", "", "", err
			}
			images = append(images, details)
		}
		version, sha, err = getLastestHelmTagandSha(images)
		if err != nil {
			return "", "", "", err
		}
		ecrname := fmt.Sprintf("%s/%s", c.ecrPublicClient.SourceRegistry, name)
		return ecrname, version, sha, err
	}
	return "", "", "", fmt.Errorf("invalid repository: %q", repoName)
}
