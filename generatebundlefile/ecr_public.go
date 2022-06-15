package main

import (
	"context"
	"fmt"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
	ecrpublictypes "github.com/aws/aws-sdk-go-v2/service/ecrpublic/types"
	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
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

// GetShaForInputs returns a list of an images version/sha for given inputs to lookup
func (c *ecrPublicClient) GetShaForPublicInputs(project Project) ([]api.SourceVersion, error) {
	sourceVersion := []api.SourceVersion{}
	for _, tag := range project.Versions {
		if !strings.Contains(tag.Name, "latest") {
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
				if len(images.ImageTags) == 1 {
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
			sha, err := getLastestImageSha(ImageDetails)
			if err != nil {
				return nil, err
			}
			sourceVersion = append(sourceVersion, *sha)
			continue
		}
		//
		if strings.Contains(tag.Name, "-latest") {
			regex := regexp.MustCompile(`-latest`)
			splitVersion := regex.Split(tag.Name, -1) //extract out the version without the latest
			ImageDetails, err := c.DescribePublic(&ecrpublic.DescribeImagesInput{
				RepositoryName: aws.String(project.Repository),
			})
			if err != nil {
				return nil, fmt.Errorf("error: Unable to complete DescribeImagesRequest to ECR public. %s", err)
			}
			filteredImageDetails := imageTagFilter(ImageDetails, splitVersion[0])
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

// getLastestImageSha Iterates list of ECR Public Helm Charts, to find latest pushed image and return tag/sha  of the latest pushed image
func getLastestImageSha(details []ecrpublictypes.ImageDetail) (*api.SourceVersion, error) {
	if len(details) == 0 {
		return nil, fmt.Errorf("no details provided")
	}
	var latest ecrpublictypes.ImageDetail
	latest.ImagePushedAt = &time.Time{}
	for _, detail := range details {
		if len(details) < 1 || detail.ImagePushedAt == nil || detail.ImageDigest == nil || detail.ImageTags == nil || len(detail.ImageTags) == 0 || *detail.ImageManifestMediaType != "application/vnd.oci.image.manifest.v1+json" {
			continue
		}
		if detail.ImagePushedAt != nil && latest.ImagePushedAt.Before(*detail.ImagePushedAt) {
			latest = detail
		}
	}
	// Check if latest is equal to empty struct, and return error if that's the case.
	if reflect.DeepEqual(latest, ecrpublictypes.ImageDetail{}) {
		return nil, fmt.Errorf("error no images found")
	}
	return &api.SourceVersion{Name: latest.ImageTags[0], Digest: *latest.ImageDigest}, nil
}

// getLastestHelmTagandShaPublic Iterates list of ECR Public Helm Charts, to find latest pushed image and return tag/sha  of the latest pushed image
func getLastestHelmTagandShaPublic(details []ecrpublictypes.ImageDetail) (string, string, error) {
	if len(details) == 0 {
		return "", "", fmt.Errorf("no details provided")
	}
	var latest ecrpublictypes.ImageDetail
	latest.ImagePushedAt = &time.Time{}
	for _, detail := range details {
		if len(details) < 1 || detail.ImagePushedAt == nil || detail.ImageDigest == nil || detail.ImageTags == nil || len(detail.ImageTags) == 0 || *detail.ImageManifestMediaType != "application/vnd.oci.image.manifest.v1+json" {
			continue
		}
		if detail.ImagePushedAt != nil && latest.ImagePushedAt.Before(*detail.ImagePushedAt) {
			latest = detail
		}
	}
	// Check if latest is equal to empty struct, and return error if that's the case.
	if reflect.DeepEqual(latest, ecrpublictypes.ImageDetail{}) {
		return "", "", fmt.Errorf("error no images found")
	}
	return latest.ImageTags[0], *latest.ImageDigest, nil
}

// imageTagFilter is used when filtering a list of images for a specific tag or tag substring
func imageTagFilter(details []ecrpublictypes.ImageDetail, version string) []ecrpublictypes.ImageDetail {
	var filteredDetails []ecrpublictypes.ImageDetail
	for _, detail := range details {
		for _, tag := range detail.ImageTags {
			if strings.Contains(tag, version) {
				filteredDetails = append(filteredDetails, detail)
			}
		}
	}
	return filteredDetails
}

// shaExistsInRepository checks if a given OCI artifact exists in a destination repo using the sha sum.
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

// shaExistsInRepository checks if a given OCI artifact exists in a destination repo using the sha sum.
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

// copyImagePrivPubSameAcct will copy an OCI artifact from ECR us-west-2 to ECR Public within the same account.
func copyImagePrivPubSameAcct(log logr.Logger, authFile, version string, stsClient *stsClient, ecrPublic *ecrPublicClient, image Image) error {
	source := fmt.Sprintf("docker://%s.dkr.ecr.us-west-2.amazonaws.com/%s:%s", stsClient.AccountID, image.Repository, image.Tag)
	destination := fmt.Sprintf("docker://%s/%s:%s", ecrPublic.SourceRegistry, image.Repository, version)
	log.Info("Promoting...", source, destination)
	cmd := exec.Command("skopeo", "copy", "--authfile", authFile, source, destination, "-f", "oci", "--all")
	_, err := ExecCommand(cmd)
	if err != nil {
		return err
	}
	return nil
}

// copyImagePubPubDifferentAcct will copy an OCI artifact from ECR Public to ECR Public to another account.
func (c *SDKClients) copyImagePubPubDifferentAcct(log logr.Logger, authFile, version string, image Image) error {
	source := fmt.Sprintf("docker://%s/%s:%s", c.ecrPublicClient.SourceRegistry, image.Repository, version)
	destination := fmt.Sprintf("docker://%s/%s:%s", c.ecrPublicClientRelease.SourceRegistry, image.Repository, version)
	log.Info("Promoting...", source, destination)
	cmd := exec.Command("skopeo", "copy", "--authfile", authFile, source, destination, "-f", "oci", "--all")
	_, err := ExecCommand(cmd)
	if err != nil {
		return err
	}
	return nil
}

// getNameAndVersionPublic looks up the latest pushed helm chart's tag from a given repo name Full name in Public ECR OCI format.
func (c *SDKClients) getNameAndVersionPublic(repoName, registryURI string) (string, string, string, error) {
	var version string
	var sha string
	splitname := strings.Split(repoName, ":") // TODO add a regex filter
	name := splitname[0]
	if len(splitname) == 1 {
		ImageDetails, err := c.ecrPublicClient.DescribePublic(&ecrpublic.DescribeImagesInput{
			RepositoryName: aws.String(repoName),
		})
		if err != nil {
			return "", "", "", err
		}
		version, sha, err = getLastestHelmTagandShaPublic(ImageDetails)
		if err != nil {
			return "", "", "", err
		}
		ecrname := fmt.Sprintf("%s/%s", c.ecrPublicClient.SourceRegistry, name)
		return ecrname, version, sha, err
	}
	version = splitname[1]
	return name, version, sha, nil
}
