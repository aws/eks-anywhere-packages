package testutil

import (
	"os"
	"path/filepath"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func GivenPod(filename string) (*v1.Pod, string, error) {
	content, err := os.ReadFile(filepath.Clean(filename) + ".signed")
	if err != nil {
		return nil, "", err
	}
	pod := &v1.Pod{}
	err = yaml.UnmarshalStrict(content, pod)
	if err != nil {
		return nil, "", err
	}
	digest, err := os.ReadFile(filepath.Clean(filename) + ".digest")
	if err != nil {
		return nil, "", err
	}
	return pod, string(digest), nil
}
