package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/pkg/errors"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

const (
	ecrRegion       = "us-west-2"
	ecrPublicRegion = "us-east-1"
)

type ecrClient struct {
	registryClient
	AuthConfig string
}

type registryClient interface {
	DescribeImages(ctx context.Context, params *ecr.DescribeImagesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error)
	GetAuthorizationToken(ctx context.Context, params *ecr.GetAuthorizationTokenInput, optFns ...func(*ecr.Options)) (*ecr.GetAuthorizationTokenOutput, error)
}

type CheckECR interface {
	tagExistsInRepository(repository, tag string) (bool, error)
	shaExistsInRepository(repository, tag string) (bool, error)
}

// NewECRClient Creates a new ECR Client client
func NewECRClient(client registryClient, needsCreds bool) (*ecrClient, error) {
	ecrClient := &ecrClient{
		registryClient: client,
	}
	if needsCreds {
		authorizationToken, err := ecrClient.GetAuthToken()
		if err != nil {
			return nil, err
		}
		ecrClient.AuthConfig = authorizationToken
		return ecrClient, nil
	}
	return ecrClient, nil
}

// Describe returns a list of ECR describe results, with Pagination from DescribeImages SDK request
func (c *ecrClient) Describe(describeInput *ecr.DescribeImagesInput) ([]ecrtypes.ImageDetail, error) {
	var images []ecrtypes.ImageDetail
	resp, err := c.DescribeImages(context.TODO(), describeInput)
	if err != nil {
		return nil, fmt.Errorf("error: Unable to complete DescribeImagesRequest to ECR. %s", err)
	}
	images = append(images, resp.ImageDetails...)
	if resp.NextToken != nil {
		next := describeInput
		next.NextToken = resp.NextToken
		nextdetails, _ := c.Describe(next)
		images = append(images, nextdetails...)
	}
	return images, nil
}

// GetShaForInputs returns a list of an images version/sha for given inputs to lookup
func (c *ecrClient) GetShaForInputs(project Project) ([]api.SourceVersion, error) {
	sourceVersion := []api.SourceVersion{}
	for _, tag := range project.Versions {
		if !strings.Contains(tag.Name, "latest") {
			var imagelookup []ecrtypes.ImageIdentifier
			imagelookup = append(imagelookup, ecrtypes.ImageIdentifier{ImageTag: &tag.Name})
			ImageDetails, err := c.Describe(&ecr.DescribeImagesInput{
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
			ImageDetails, err := c.Describe(&ecr.DescribeImagesInput{
				RepositoryName: aws.String(project.Repository),
			})
			if err != nil {
				return nil, fmt.Errorf("error: Unable to complete DescribeImagesRequest to ECR public. %s", err)
			}
			var images []ImageDetailsBothECR
			for _, image := range ImageDetails {
				details, _ := createECRImageDetails(ImageDetailsECR{PrivateImageDetails: image})
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
		if strings.Contains(tag.Name, "-latest") {
			regex := regexp.MustCompile(`-latest`)
			splitVersion := regex.Split(tag.Name, -1) //extract out the version without the latest
			ImageDetails, err := c.Describe(&ecr.DescribeImagesInput{
				RepositoryName: aws.String(project.Repository),
			})
			if err != nil {
				return nil, fmt.Errorf("error: Unable to complete DescribeImagesRequest to ECR public. %s", err)
			}
			var images []ImageDetailsBothECR
			for _, image := range ImageDetails {
				details, _ := createECRImageDetails(ImageDetailsECR{PrivateImageDetails: image})
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

// tagFromSha Looks up the Tag of an ECR artifact from a sha
func (c *ecrClient) tagFromSha(repository, sha string) (string, error) {
	if repository == "" || sha == "" {
		return "", fmt.Errorf("Emtpy repository, or sha passed to the function")
	}
	var imagelookup []ecrtypes.ImageIdentifier
	imagelookup = append(imagelookup, ecrtypes.ImageIdentifier{ImageDigest: &sha})
	ImageDetails, err := c.Describe(&ecr.DescribeImagesInput{
		RepositoryName: aws.String(repository),
		ImageIds:       imagelookup,
	})
	if err != nil {
		if strings.Contains(err.Error(), "does not exist within the repository") == true {
			return "", nil
		} else {
			return "", fmt.Errorf("looking up image details %v", err)
		}
	}
	for _, detail := range ImageDetails {
		// We can return the first tag for an image, if it has multiple tags
		if len(detail.ImageTags) > 0 {
			detail.ImageTags = removeStringSlice(detail.ImageTags, "latest")
			return detail.ImageTags[0], nil
		}
	}
	return "", nil
}

// GetAuthToken gets an authorization token from ECR
func (c *ecrClient) GetAuthToken() (string, error) {
	authTokenOutput, err := c.GetAuthorizationToken(context.TODO(), &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return "", errors.Cause(err)
	}
	authToken := *authTokenOutput.AuthorizationData[0].AuthorizationToken
	return authToken, nil
}

//NewAuthFile writes a new Docker Authfile from the DockerAuth struct which a user to be used by Skopeo or Helm.
func NewAuthFile(dockerstruct *DockerAuth) (DockerAuthFile, error) {
	jsonbytes, err := json.Marshal(*dockerstruct)
	auth := DockerAuthFile{}
	if err != nil {
		return auth, fmt.Errorf("Marshalling docker auth file to json %w", err)
	}
	f, err := os.CreateTemp("", "dockerAuth")
	if err != nil {
		return auth, fmt.Errorf("Creating tempfile %w", err)
	}
	defer f.Close()
	fmt.Fprint(f, string(jsonbytes))
	auth.Authfile = f.Name()
	return auth, nil
}

func (d DockerAuthFile) Remove() error {
	if d.Authfile == "" {
		return fmt.Errorf("No Authfile in DockerAuthFile given")
	}
	defer os.Remove(d.Authfile)
	return nil
}

// getNameAndVersionPrivate looks up the latest pushed helm chart's tag from a given repo name from ECR.
func (c *SDKClients) getNameAndVersion(s, accountID string) (string, string, string, error) {
	var version string
	var sha string
	splitname := strings.Split(s, ":") // TODO add a regex filter
	name := splitname[0]
	if len(splitname) == 1 {
		ImageDetails, err := c.ecrClient.Describe(&ecr.DescribeImagesInput{
			RepositoryName: aws.String(s),
		})
		if err != nil {
			return "", "", "", err
		}
		var images []ImageDetailsBothECR
		for _, image := range ImageDetails {
			details, err := createECRImageDetails(ImageDetailsECR{PrivateImageDetails: image})
			if err != nil {
				return "", "", "", err
			}
			images = append(images, details)
		}
		version, sha, err = getLastestHelmTagandSha(images)
		ecrname := fmt.Sprintf("%s.dkr.ecr.us-west-2.amazonaws.com/%s", accountID, name)
		return ecrname, version, sha, err
	}
	version = splitname[1]
	return name, version, sha, nil
}

// shaExistsInRepository checks if a given OCI artifact exists in a destination repo using the sha sum.
func (c *ecrClient) shaExistsInRepository(repository, sha string) (bool, error) {
	if repository == "" || sha == "" {
		return false, fmt.Errorf("Emtpy repository, or sha passed to the function")
	}
	var imagelookup []ecrtypes.ImageIdentifier
	imagelookup = append(imagelookup, ecrtypes.ImageIdentifier{ImageDigest: &sha})
	ImageDetails, err := c.Describe(&ecr.DescribeImagesInput{
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
func (c *ecrClient) tagExistsInRepository(repository, tag string) (bool, error) {
	if repository == "" || tag == "" {
		return false, fmt.Errorf("Emtpy repository, or tag passed to the function")
	}
	var imagelookup []ecrtypes.ImageIdentifier
	imagelookup = append(imagelookup, ecrtypes.ImageIdentifier{ImageTag: &tag})
	ImageDetails, err := c.Describe(&ecr.DescribeImagesInput{
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
