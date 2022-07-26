package main

import (
	"fmt"
	"os/exec"
	"strings"

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
