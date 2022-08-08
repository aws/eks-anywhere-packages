package file

import (
	"fmt"
	"github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"testing"
)

func givenFile(file string, config KindAccessor) error {
	reader := NewFileReader(file)
	err := reader.Initialize(config)
	if err != nil {
		return err
	}
	return reader.Parse(config)
}

func GivenPackage(fileName string) (*v1alpha1.Package, error) {
	config := &v1alpha1.Package{}
	err := givenFile(fileName, config)
	return config, err
}

func GivenPackageBundle(filename string) (*v1alpha1.PackageBundle, error) {
	config := &v1alpha1.PackageBundle{}
	reader := NewFileReader(filename + ".signed")
	initError := reader.Initialize(config)
	if initError != nil {
		return nil, initError
	}
	actual := reader.Parse(config)
	if actual != nil {
		return nil, actual
	}
	return config, nil
}

func GivenBundleController(fileName string) (*v1alpha1.PackageBundleController, error) {
	config := &v1alpha1.PackageBundleController{}
	err := givenFile(fileName, config)
	return config, err
}

// MustPackageBundleFromFilename is a helper to load a bundle or fail trying.
//
// It is intended primarily for use in automated tests or utilities.
func MustPackageBundleFromFilename(t *testing.T, filename string) (bundle *v1alpha1.PackageBundle) {
	bundle = &v1alpha1.PackageBundle{}
	r := NewFileReader(filename)
	err := r.Initialize(bundle)
	if err != nil {
		t.Fatalf(fmt.Sprintf("initializing YAML FileReader: %s", err))
	}

	err = r.Parse(bundle)
	if err != nil {
		t.Fatalf(fmt.Sprintf("parsing YAML file: %s", err))
	}

	return bundle
}
