package main

import (
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"time"

	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	ecrpublictypes "github.com/aws/aws-sdk-go-v2/service/ecrpublic/types"
	"github.com/go-logr/logr"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

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

func deleteEmptyStringSlice(s []string) []string {
	var r []string
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}

func printSlice(s []string) {
	if len(s) == 0 {
		return
	}
	fmt.Println(s[0])
	printSlice(s[1:])
}

func printMap(s map[string]string) {
	if len(s) == 0 {
		return
	}
	for k, v := range s {
		fmt.Println(k, ":", v)
	}
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
		return strings.Join(chartNameList[2:], "/"), chartNameList[len(chartNameList)-1], nil
	}
	if len(chartNameList) > 1 {
		return strings.Join(chartNameList[1:], "/"), chartNameList[len(chartNameList)-1], nil
	}
	return "", "", fmt.Errorf("Error: %s", "Failed parsing chartName, check the input URI is a valid ECR URI")
}

// imageTagFilter is used when filtering a list of images for a specific tag or tag substring
func ImageTagFilter(details []ImageDetailsBothECR, version string) []ImageDetailsBothECR {
	var filteredDetails []ImageDetailsBothECR
	for _, detail := range details {
		for _, tag := range detail.ImageTags {
			if strings.Contains(tag, version) {
				filteredDetails = append(filteredDetails, detail)
			}
		}
	}
	return filteredDetails
}

type ImageDetailsECR struct {
	PrivateImageDetails ecrtypes.ImageDetail
	PublicImageDetails  ecrpublictypes.ImageDetail
}

// ImageDetailsBothECR is used so we can share some functions between private and public ECR bundle creation.
type ImageDetailsBothECR struct {
	// The sha256 digest of the image manifest.
	ImageDigest *string `copier:"ImageDigest"`

	// The media type of the image manifest.
	ImageManifestMediaType *string `copier:"ImageManifestMediaType"`

	// The date and time, expressed in standard JavaScript date format, at which the
	// current image was pushed to the repository.
	ImagePushedAt *time.Time `copier:"ImagePushedAt"`

	// The list of tags associated with this image.
	ImageTags []string `copier:"ImageTags"`

	// The Amazon Web Services account ID associated with the registry to which this
	// image belongs.
	RegistryId *string `copier:"RegistryId"`

	// The name of the repository to which this image belongs.
	RepositoryName *string `copier:"RepositoryName"`
}

func createECRImageDetails(images ImageDetailsECR) (ImageDetailsBothECR, error) {
	t := &ImageDetailsBothECR{}
	//Check for empty structs, if non empty copy to new common struct for ECR imagedetails.
	if reflect.DeepEqual(images.PublicImageDetails, ecrpublictypes.ImageDetail{}) {
		if images.PrivateImageDetails.ImageDigest != nil && images.PrivateImageDetails.ImagePushedAt != nil && images.PrivateImageDetails.RegistryId != nil && images.PrivateImageDetails.RepositoryName != nil {
			copier.Copy(&t, &images.PrivateImageDetails)
			return *t, nil
		}
		return ImageDetailsBothECR{}, fmt.Errorf("Error marshalling image details from ECR lookup.")
	}
	if reflect.DeepEqual(images.PrivateImageDetails, ecrtypes.ImageDetail{}) {
		if images.PublicImageDetails.ImageDigest != nil && images.PublicImageDetails.ImagePushedAt != nil && images.PublicImageDetails.RegistryId != nil && images.PublicImageDetails.RepositoryName != nil {
			copier.Copy(&t, &images.PublicImageDetails)
			return *t, nil
		}
		return ImageDetailsBothECR{}, fmt.Errorf("Error marshalling image details from ECR lookup.")
	}
	return ImageDetailsBothECR{}, fmt.Errorf("Error no data passed to createImageDetails")
}

// getLastestHelmTagandSha Iterates list of ECR Helm Charts, to find latest pushed image and return tag/sha  of the latest pushed image
func getLastestHelmTagandSha(details []ImageDetailsBothECR) (string, string, error) {
	var latest ImageDetailsBothECR
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
	if reflect.DeepEqual(latest, ImageDetailsBothECR{}) {
		return "", "", fmt.Errorf("error no images found")
	}
	return latest.ImageTags[0], *latest.ImageDigest, nil
}

// getLastestImageSha Iterates list of ECR Public Helm Charts, to find latest pushed image and return tag/sha  of the latest pushed image
func getLastestImageSha(details []ImageDetailsBothECR) (*api.SourceVersion, error) {
	var latest ImageDetailsBothECR
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

// copyImage will copy an OCI artifact from one registry to another registry.
func copyImage(log logr.Logger, authFile, source, destination string) error {
	log.Info("Running skopeo copy...", source, destination)
	cmd := exec.Command("skopeo", "copy", "--authfile", authFile, source, destination, "-f", "oci", "--all")
	stdout, err := ExecCommand(cmd)
	fmt.Printf("%s\n", stdout)
	if err != nil {
		return err
	}
	return nil
}
