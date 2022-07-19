package testutil

import (
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

func GivenPackageBundleController(filename string) (*v1alpha1.PackageBundleController, error) {
	content, err := os.ReadFile(filepath.Clean(filename))
	if err != nil {
		return nil, err
	}
	pbc := &v1alpha1.PackageBundleController{}
	err = yaml.UnmarshalStrict(content, pbc)
	if err != nil {
		return nil, err
	}
	return pbc, nil
}

func GivenPackage(filename string) (*v1alpha1.Package, error) {
	content, err := os.ReadFile(filepath.Clean(filename))
	if err != nil {
		return nil, err
	}
	p := &v1alpha1.Package{}
	err = yaml.UnmarshalStrict(content, p)
	if err != nil {
		return nil, err
	}
	return p, nil
}
