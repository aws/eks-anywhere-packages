package v1alpha1

import (
	"testing"

	"github.com/aws/eks-anywhere-packages/api"
)

// These tests ensure that the api.FileReader can correctly unmarshal the API
// types we've defined.

func givenFile(file string, config api.KindAccessor) error {
	reader := api.NewFileReader(file)
	err := reader.Initialize(config)
	if err != nil {
		return err
	}
	return reader.Parse(config)
}

func GivenBundleController(config *PackageBundleController) error {
	err := givenFile("../testdata/packagebundlecontroller.yaml", config)
	return err
}

func GivenPackageBundleOne(config *PackageBundle) error {
	err := givenFile("../testdata/bundle_one.yaml", config)
	return err
}

func GivenPackageBundleTwo(config *PackageBundle) error {
	err := givenFile("../testdata/bundle_two.yaml", config)
	return err
}

func GivenPackage(config *Package) error {
	err := givenFile("../testdata/test.yaml", config)
	return err
}

// TestFileReaderOnApiDatatypes tests that the file reader can correctly
// unmarshal our CRD types.
func TestFileReaderOnApiDatatypes(t *testing.T) {
	var actual error
	var bundleController PackageBundleController
	actual = GivenBundleController(&bundleController)
	if actual != nil {
		t.Errorf("expected <%v> actual <%v>", nil, actual)
	}

	var bundle PackageBundle
	actual = GivenPackageBundleTwo(&bundle)
	if actual != nil {
		t.Errorf("expected <%v> actual <%v>", nil, actual)
	}

	actual = GivenPackageBundleOne(&bundle)
	if actual != nil {
		t.Errorf("expected <%v> actual <%v>", nil, actual)
	}

	var test Package
	actual = GivenPackage(&test)
	if actual != nil {
		t.Errorf("expected <%v> actual <%v>", nil, actual)
	}
}
