package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/pkg/errors"
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

// getLastestHelmTagandSha Iterates list of ECR Helm Charts, to find latest pushed image and return tag/sha  of the latest pushed image
func getLastestHelmTagandSha(details []ecrtypes.ImageDetail) (string, string, error) {
	if len(details) == 0 {
		return "", "", fmt.Errorf("no details provided")
	}
	var latest ecrtypes.ImageDetail
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
	if reflect.DeepEqual(latest, ecrtypes.ImageDetail{}) {
		return "", "", fmt.Errorf("error no images found")
	}
	return latest.ImageTags[0], *latest.ImageDigest, nil
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
	//defer os.Remove(d.Authfile)
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
		version, sha, err = getLastestHelmTagandSha(ImageDetails)
		if err != nil {
			return "", "", "", err
		}
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
