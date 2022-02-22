package v1alpha1

import (
	"fmt"
	"testing"

	"github.com/aws/eks-anywhere-packages/api"
)

// MustPackageBundleFromFilename is a helper to load a bundle or fail trying.
//
// It is intended primarily for use in automated tests or utilities.
func MustPackageBundleFromFilename(t *testing.T, filename string) (bundle *PackageBundle) {
	bundle = &PackageBundle{}
	r := api.NewFileReader(filename)
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
