package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
	ecrpublictypes "github.com/aws/aws-sdk-go-v2/service/ecrpublic/types"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

const (
	ecrRegion       = "us-west-2"
	ecrPublicRegion = "us-east-1"
)

type ecrPublicClient struct {
	*ecrpublic.Client
	AuthConfig     string
	SourceRegistry string
}

type ecrClient struct {
	*ecr.Client
	AuthConfig string
}

// NewECRPublicClient Creates a new ECR Client Public client
func NewECRPublicClient(needsCreds bool, conf *aws.Config) (*ecrPublicClient, error) {
	if conf == nil {
		loadConfig, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(ecrPublicRegion))
		if err != nil {
			return nil, fmt.Errorf("loading default ECR public config %w", err)
		}
		conf = &loadConfig
	}
	ecrPublicClient := &ecrPublicClient{
		Client: ecrpublic.NewFromConfig(*conf),
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

// NewECRClient Creates a new ECR Client Public client
func NewECRClient(creds bool) (*ecrClient, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(ecrRegion))
	if err != nil {
		return nil, fmt.Errorf("Creating AWS ECR config %w", err)
	}
	ecrClient := &ecrClient{Client: ecr.NewFromConfig(cfg)}
	if creds {
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
func (c *ecrPublicClient) GetShaForInputs(project Project) ([]api.SourceVersion, error) {
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

// removeDuplicates removes any duplicates from Version list, useful for scenarios when
// multiple tags for an image are present, this would cause duplicates on the bundle CRD,
// so we remove the first one in this case since they are the same thing.
// EX sha1234 is tagged 1.1 and 1.2 and sha5678 is tagged 1.2 this would result in a double match of 1.2 so we run this.
func removeDuplicates(s []api.SourceVersion) []api.SourceVersion {
	k := make(map[string]bool)
	l := []api.SourceVersion{}
	for _, i := range s {
		if _, j := k[i.Name]; !j {
			k[i.Name] = true
			l = append(l, api.SourceVersion{Name: i.Name, Digest: i.Digest})
		}
	}
	return l
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
	if reflect.DeepEqual(latest, ecrtypes.ImageDetail{}) {
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

// tagFromSha Looks up the Tag of an ECR artifact from a sha
func (c *ecrClient) tagFromSha(repository, sha string) (string, error) {
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

// stringInSlice checks to see if a string is in a slice
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// removeStringSlice removes a named string from a slice, without knowing it's index or it being ordered.
func removeStringSlice(l []string, item string) []string {
	for i, other := range l {
		if other == item {
			return append(l[:i], l[i+1:]...)
		}
	}
	return l
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

// GetAuthToken gets an authorization token from ECR
func (c *ecrClient) GetAuthToken() (string, error) {
	authTokenOutput, err := c.GetAuthorizationToken(context.TODO(), &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return "", errors.Cause(err)
	}
	authToken := *authTokenOutput.AuthorizationData[0].AuthorizationToken
	return authToken, nil
}

type DockerAuth struct {
	Auths map[string]DockerAuthRegistry `json:"auths,omitempty"`
}

type DockerAuthRegistry struct {
	Auth string `json:"auth"`
}

//NewAuthFile writes a new Docker Authfile from the DockerAuth struct which a user to be used by Skopeo or Helm.
func NewAuthFile(dockerstruct *DockerAuth) (string, error) {
	jsonbytes, err := json.Marshal(*dockerstruct)
	if err != nil {
		return "", fmt.Errorf("Marshalling docker auth file to json %w", err)
	}
	f, err := os.CreateTemp("", "dockerAuth")
	if err != nil {
		return "", fmt.Errorf("Creating tempfile %w", err)
	}
	defer f.Close()
	fmt.Fprint(f, string(jsonbytes))
	return f.Name(), nil
}

// copyImagePrivPubSameAcct will copy an OCI artifact from ECR us-west-2 to ECR Public within the same account.
func copyImagePrivPubSameAcct(log logr.Logger, authFile string, stsClient *stsClient, ecrPublic *ecrPublicClient, image Image) error {
	source := fmt.Sprintf("docker://%s.dkr.ecr.us-west-2.amazonaws.com/%s:%s", stsClient.AccountID, image.Repository, image.Tag)
	destination := fmt.Sprintf("docker://%s/%s:%s", ecrPublic.SourceRegistry, image.Repository, image.Tag)
	log.Info("Promoting...", source, destination)
	cmd := exec.Command("skopeo", "copy", "--authfile", authFile, source, destination, "-f", "oci", "--all")
	_, err := ExecCommand(cmd)
	if err != nil {
		return err
	}
	return nil
}

// copyImagePubPubDifferentAcct will copy an OCI artifact from ECR Public to ECR Public to another account.
func (c *SDKClients) copyImagePubPubDifferentAcct(log logr.Logger, authFile string, image Image) error {
	source := fmt.Sprintf("docker://%s/%s:%s", c.ecrPublicClient.SourceRegistry, image.Repository, image.Tag)
	destination := fmt.Sprintf("docker://%s/%s:%s", c.ecrPublicClientRelease.SourceRegistry, image.Repository, image.Tag)
	log.Info("Promoting...", source, destination)
	cmd := exec.Command("skopeo", "copy", "--authfile", authFile, source, destination, "-f", "oci", "--all")
	_, err := ExecCommand(cmd)
	if err != nil {
		return err
	}
	return nil
}

// ExecCommand runs a given command, and constructs the log/output.
func ExecCommand(cmd *exec.Cmd) (string, error) {
	stdout, err := cmd.Output()
	if err != nil {
		return string(stdout), errors.Cause(err)
	}
	return string(stdout), nil
}

// splitECRName is a helper function where some ECR repo's are formatted with "org/repo", and for aws repos it's just "repo"
func splitECRName(s string) (string, string, error) {
	chartNameList := strings.Split(s, "/")
	// Scenarios for ECR Public which contain and extra "/"
	if strings.Contains(chartNameList[0], "public.ecr.aws") {
		if len(chartNameList) == 3 {
			return chartNameList[2], chartNameList[2], nil
		}
		// Scenario's where we use Public ECR it's adding an extra /
		if len(chartNameList) == 4 {
			return fmt.Sprintf("%s/%s", chartNameList[2], chartNameList[3]), chartNameList[3], nil
		}
	}
	if len(chartNameList) == 2 {
		return chartNameList[1], chartNameList[1], nil
	}
	if len(chartNameList) == 3 {
		return fmt.Sprintf("%s/%s", chartNameList[1], chartNameList[2]), chartNameList[2], nil
	}
	return "", "", fmt.Errorf("Error: %s", "Failed parsing chartName, check the input URI is a valid ECR URI")
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
