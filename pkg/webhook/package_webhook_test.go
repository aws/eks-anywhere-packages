package webhook

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/eks-anywhere-packages/pkg/testutil"
)

func TestPackageValidate(t *testing.T) {
	t.Run("valid package", func(t *testing.T) {
		activeBundle, err := testutil.GivenPackageBundle("../../api/testdata/bundle_one.yaml")
		require.Nil(t, err)
		myPackage, err := testutil.GivenPackage("../../api/testdata/package_webhook_valid_config.yaml")
		require.Nil(t, err)
		validator := packageValidator{}

		result, err := validator.isPackageValid(myPackage, activeBundle)

		assert.True(t, result)
		assert.Nil(t, err)
	})

	t.Run("invalid package config", func(t *testing.T) {
		activeBundle, err := testutil.GivenPackageBundle("../../api/testdata/bundle_one.yaml")
		require.Nil(t, err)
		myPackage, err := testutil.GivenPackage("../../api/testdata/package_webhook_invalid_config.yaml")
		require.Nil(t, err)
		validator := packageValidator{}

		result, err := validator.isPackageValid(myPackage, activeBundle)

		assert.False(t, result)
		assert.EqualError(t, err, "error validating configurations - (root): Additional property fakeConfig is not allowed\n")
	})

	t.Run("invalid package config type", func(t *testing.T) {
		activeBundle, err := testutil.GivenPackageBundle("../../api/testdata/bundle_one.yaml")
		require.Nil(t, err)
		myPackage, err := testutil.GivenPackage("../../api/testdata/package_webhook_invalid_type.yaml")
		require.Nil(t, err)
		validator := packageValidator{}

		result, err := validator.isPackageValid(myPackage, activeBundle)

		assert.False(t, result)
		assert.EqualError(t, err, "error validating configurations - title: Invalid type. Expected: string, given: integer\n")
	})

	t.Run("status targetNamespace not set", func(t *testing.T) {
		activeBundle, err := testutil.GivenPackageBundle("../../api/testdata/bundle_one.yaml")
		require.Nil(t, err)
		myPackage, err := testutil.GivenPackage("../../api/testdata/package_webhook_valid_config.yaml")
		require.Nil(t, err)
		myPackage.Spec.TargetNamespace = "default"
		myPackage.Status.Spec.TargetNamespace = ""
		validator := packageValidator{}

		result, err := validator.isPackageValid(myPackage, activeBundle)

		assert.True(t, result)
		assert.Nil(t, err)
	})

	t.Run("status targetNamespace same", func(t *testing.T) {
		activeBundle, err := testutil.GivenPackageBundle("../../api/testdata/bundle_one.yaml")
		require.Nil(t, err)
		myPackage, err := testutil.GivenPackage("../../api/testdata/package_webhook_valid_config.yaml")
		require.Nil(t, err)
		myPackage.Spec.TargetNamespace = "default"
		myPackage.Status.Spec.TargetNamespace = "default"
		validator := packageValidator{}

		result, err := validator.isPackageValid(myPackage, activeBundle)

		assert.True(t, result)
		assert.Nil(t, err)
	})

	t.Run("status targetNamespace not same", func(t *testing.T) {
		activeBundle, err := testutil.GivenPackageBundle("../../api/testdata/bundle_one.yaml")
		require.Nil(t, err)
		myPackage, err := testutil.GivenPackage("../../api/testdata/package_webhook_valid_config.yaml")
		require.Nil(t, err)
		myPackage.Spec.TargetNamespace = "new-namespace"
		myPackage.Status.Spec.TargetNamespace = "default"
		validator := packageValidator{}

		result, err := validator.isPackageValid(myPackage, activeBundle)

		assert.False(t, result)
		assert.EqualError(t, err, "package my-hello-eks-anywhere targetNamespace is immutable")
	})
}
