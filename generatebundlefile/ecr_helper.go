package main

import (
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"time"

	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
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
	if len(chartNameList) > 1 {
		return strings.Join(chartNameList[1:], "/"), chartNameList[len(chartNameList)-1], nil
	}
	return "", "", fmt.Errorf("parsing chartName, check the input URI is a valid ECR URI")
}

// imageTagFilter is used when filtering a list of images for a specific tag or tag substring
func ImageTagFilter(details []ecrtypes.ImageDetail, version string) []ecrtypes.ImageDetail {
	var filteredDetails []ecrtypes.ImageDetail
	for _, detail := range details {
		for _, tag := range detail.ImageTags {
			if strings.HasPrefix(tag, version) && strings.Contains(tag, "latest") {
				filteredDetails = append(filteredDetails, detail)
			}
		}
	}
	return filteredDetails
}

// getLatestImageSha Iterates list of Helm Charts, to find latest pushed image and return tag/sha  of the latest pushed image
func getLatestImageSha(details []ecrtypes.ImageDetail) (*api.SourceVersion, error) {
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
		return nil, fmt.Errorf("error no images found")
	}
	return &api.SourceVersion{Name: latest.ImageTags[0], Digest: *latest.ImageDigest}, nil
}
