package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	ecrpublictypes "github.com/aws/aws-sdk-go-v2/service/ecrpublic/types"
	"github.com/go-logr/logr"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

const imageIndexMediaType = "application/vnd.oci.image.index.v1+json"

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

func printSlice[v any](s []v) {
	if len(s) == 0 {
		return
	}
	fmt.Println(s[0])
	printSlice(s[1:])
}

func printMap[k comparable, v any](m map[k]v) {
	if len(m) == 0 {
		return
	}
	for k, v := range m {
		fmt.Println(k, ":", v)
	}
}

// ExecCommand runs a given command, and constructs the log/output.
func ExecCommand(cmd *exec.Cmd) (string, error) {
	commandOutput, err := cmd.CombinedOutput()
	commandOutputStr := strings.TrimSpace(string(commandOutput))
	if err != nil {
		return commandOutputStr, errors.Cause(err)
	}
	return commandOutputStr, nil
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
			if strings.HasPrefix(tag, version) {
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
	// Check for empty structs, if non empty copy to new common struct for ECR imagedetails.
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

// getLatestHelmTagandSha Iterates list of ECR Helm Charts, to find latest pushed image and return tag/sha  of the latest pushed image
func getLatestHelmTagandSha(details []ImageDetailsBothECR) (string, string, error) {
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

	var imageTag string
	imageTagRegex := regexp.MustCompile(`^\d+\.\d+\.\d+.*[0-9a-f]{40}$`)
	for _, tag := range latest.ImageTags {
		if imageTagRegex.MatchString(tag) {
			imageTag = tag
			break
		}
	}
	return imageTag, *latest.ImageDigest, nil
}

// getLatestImageSha Iterates list of ECR Public Helm Charts, to find latest pushed image and return tag/sha  of the latest pushed image
func getLatestImageSha(details []ImageDetailsBothECR) (*api.SourceVersion, error) {
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
	// Create temporary directory for copying image artifacts locally
	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}
	tempImageDir := filepath.Join(currentDir, "temp-image-dir")
	defer os.RemoveAll(tempImageDir)

	// Copy image from source registry to local directory
	log.Info("Copying source image to local directory", "Source", source, "Directory", fmt.Sprintf("dir://%s", tempImageDir))
	cmd := exec.Command("skopeo", "copy", "--authfile", authFile, source, fmt.Sprintf("dir://%s", tempImageDir), "-f", "oci", "--all")
	stdout, err := ExecCommand(cmd)
	fmt.Printf("%s\n", stdout)
	if err != nil {
		return err
	}

	manifestFile := filepath.Join(tempImageDir, "manifest.json")

	// Fetch manifest media type from manifest.json present inside the
	// copied image directory
	log.Info("Getting media type from root manifest JSON")
	cmd = exec.Command("bash", "-c", fmt.Sprintf("cat %s | jq -r '.mediaType'", manifestFile))
	mediaType, err := ExecCommand(cmd)
	if err != nil {
		return err
	}

	if mediaType == imageIndexMediaType {
		// Remove manifest.json files corresponding to all artifacts that are //not images.
		// These might be SBOMs, attributions which `skopeo copy` cannot handle. We filter
		// these out by checking for artifacts that have an "unknown" architecture value.
		log.Info("Removing manifest JSON files for all non-image artifacts")
		log.Info("Getting non-image artifact digests")
		cmd = exec.Command("bash", "-c", fmt.Sprintf("cat %s | jq -r '.manifests[] | select(.platform.architecture == \"unknown\").digest'", manifestFile))
		stdout, err = ExecCommand(cmd)
		if err != nil {
			return err
		}
		nonImageDigests := strings.Split(stdout, "\n")
		for _, digest := range nonImageDigests {
			if digest != "" {
				trimmedDigest := strings.TrimPrefix(digest, "sha256:")
				manifestFile := filepath.Join(tempImageDir, fmt.Sprintf("%s.manifest.json", trimmedDigest))
				err = os.Remove(manifestFile)
				if err != nil {
					return err
				}
			}
		}

		// Next we move on to the gzipped files representing artifact layers. We need to delete
		// the layer files for all artifacts expect those corresponding to the images. So we filter
		// each artifact that is of image type and compile the list of layer files to retain. Then we
		// iterate over all the files in the local directory and delete everything except this list.
		log.Info("Removing compressed layer files for all non-image artifacts")
		filesToRetain := []string{"manifest.json", "version"}
		log.Info("Getting image artifact digests")
		cmd = exec.Command("bash", "-c", fmt.Sprintf("cat %s | jq -r '.manifests[] | select(.platform.architecture != \"unknown\").digest'", manifestFile))
		stdout, err = ExecCommand(cmd)
		if err != nil {
			return err
		}
		imageDigests := strings.Split(stdout, "\n")
		for _, digest := range imageDigests {
			trimmedDigest := strings.TrimPrefix(digest, "sha256:")
			filesToRetain = append(filesToRetain, fmt.Sprintf("%s.manifest.json", trimmedDigest))
			manifestFile := filepath.Join(tempImageDir, fmt.Sprintf("%s.manifest.json", trimmedDigest))
			log.Info("Getting config digest for image")
			cmd = exec.Command("bash", "-c", fmt.Sprintf("cat %s | jq -r '.config.digest'", manifestFile))
			stdout, err = ExecCommand(cmd)
			if err != nil {
				return err
			}
			configDigest := strings.TrimPrefix(stdout, "sha256:")
			filesToRetain = append(filesToRetain, configDigest)

			log.Info("Getting layer digests for image")
			cmd = exec.Command("bash", "-c", fmt.Sprintf("cat %s | jq -r '.layers[].digest'", manifestFile))
			stdout, err = ExecCommand(cmd)
			if err != nil {
				return err
			}
			layerDigests := strings.Split(stdout, "\n")
			for _, digest := range layerDigests {
				layerDigest := strings.TrimPrefix(digest, "sha256:")
				if !slices.Contains(filesToRetain, layerDigest) {
					filesToRetain = append(filesToRetain, layerDigest)
				}
			}
		}

		tempImageDirFiles, err := os.ReadDir(tempImageDir)
		if err != nil {
			return err
		}
		for _, file := range tempImageDirFiles {
			if !slices.Contains(filesToRetain, file.Name()) {
				err = os.Remove(filepath.Join(tempImageDir, file.Name()))
				if err != nil {
					return err
				}
			}
		}

		// Finally we update the root manifest.json to include only the image artifacts
		// by deleting all other media types.
		log.Info("Updating root manifest JSON contents to remove all non-image artifacts")
		cmd = exec.Command("bash", "-c", fmt.Sprintf("cat %s | jq 'del(.manifests[] | select(.platform.architecture == \"unknown\"))'", manifestFile))
		updatedManifestContents, err := ExecCommand(cmd)
		if err != nil {
			return err
		}

		err = os.WriteFile(manifestFile, []byte(updatedManifestContents), 0o644)
		if err != nil {
			return err
		}

		// When using digest references as URIs, Skopeo complains if the manifest digest
		// does not not match the destination reference. So we update the destination
		// digest reference to the actual digest of the manifest to avoid this issue.
		if strings.Contains(destination, "@sha256:") {
			imageDigestRegex := regexp.MustCompile("sha256:.*")
			h := sha256.New()
			h.Write([]byte(updatedManifestContents))
			updatedManifestDigest := fmt.Sprintf("%x", h.Sum(nil))
			destination = imageDigestRegex.ReplaceAllString(destination, fmt.Sprintf("sha256:%s", updatedManifestDigest))
		}
	}

	// Copy image from local directory to destination registry
	log.Info("Copying image from local directory to destination", "Directory", fmt.Sprintf("dir://%s", tempImageDir), "Destination", destination)
	cmd = exec.Command("skopeo", "copy", "--authfile", authFile, fmt.Sprintf("dir://%s", tempImageDir), destination, "-f", "oci", "--all")
	stdout, err = ExecCommand(cmd)
	fmt.Printf("%s\n", stdout)
	if err != nil {
		return err
	}
	return nil
}
