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

func GivenBundleController(fileName string) (*PackageBundleController, error) {
	config := &PackageBundleController{}
	err := givenFile(fileName, config)
	return config, err
}

func GivenPackageBundleOne(config *PackageBundle) error {
	err := givenFile("../testdata/bundle_one.yaml", config)
	return err
}

func GivenPackageBundleTwo(config *PackageBundle) error {
	err := givenFile("../testdata/bundle_two.yaml", config)
	return err
}

func GivenPackage(fileName string) (*Package, error) {
	config := &Package{}
	err := givenFile(fileName, config)
	return config, err
}

// TestFileReaderOnApiDatatypes tests that the file reader can correctly
// unmarshal our CRD types.
func TestFileReaderOnApiDatatypes(t *testing.T) {
	_, err := GivenBundleController("../testdata/packagebundlecontroller.yaml")
	if err != nil {
		t.Errorf("expected <%v> actual <%v>", nil, err)
	}

	var bundle PackageBundle
	err = GivenPackageBundleTwo(&bundle)
	if err != nil {
		t.Errorf("expected <%v> actual <%v>", nil, err)
	}

	err = GivenPackageBundleOne(&bundle)
	if err != nil {
		t.Errorf("expected <%v> actual <%v>", nil, err)
	}

	_, err = GivenPackage("../testdata/test.yaml")
	if err != nil {
		t.Errorf("expected <%v> actual <%v>", nil, err)
	}
}
