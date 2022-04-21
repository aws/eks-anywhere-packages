package testutil

import (
	"os"
	"path/filepath"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/aws/eks-anywhere-packages/api"
	"github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

func GivenPackageBundle(filename string) (*v1alpha1.PackageBundle, string, error) {
	config := &v1alpha1.PackageBundle{}
	reader := api.NewFileReader(filename + ".signed")
	initError := reader.Initialize(config)
	if initError != nil {
		return nil, "", initError
	}
	actual := reader.Parse(config)
	if actual != nil {
		return nil, "", actual
	}
	digest, err := os.ReadFile(filepath.Clean(filename) + ".digest")
	if err != nil {
		return nil, "", err
	}
	return config, string(digest), nil
}

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
