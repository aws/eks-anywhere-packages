package v1alpha1_test

import (
	"testing"

	"github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

func TestPackageController_ExpectedKind(t *testing.T) {
	sut := v1alpha1.PackageController{}

	expected := "PackageController"
	if sut.ExpectedKind() != expected {
		t.Errorf("expected <%v> actual <%v>", expected, sut.ExpectedKind())
	}
}
