package webhook

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/testutil"
)

func TestBundleValidate(t *testing.T) {

	t.Run("happy case", func(t *testing.T) {
		myBundle, err := testutil.GivenPackageBundle("../../api/testdata/prod_bundle.yaml")
		require.Nil(t, err)

		err = myBundle.BundleValidate()

		assert.Nil(t, err)
	})

	t.Run("missing signature", func(t *testing.T) {
		myBundle := v1alpha1.PackageBundle{ObjectMeta: metav1.ObjectMeta{Name: "v1-21-003"}}

		err := myBundle.BundleValidate()

		assert.EqualError(t, err, "Missing signature")
	})

	t.Run("invalid name", func(t *testing.T) {
		myBundle := v1alpha1.PackageBundle{ObjectMeta: metav1.ObjectMeta{Name: "kevin-morby"}}

		err := myBundle.BundleValidate()

		assert.EqualError(t, err, "Invalid bundle name (should be in the format vx-xx-xxxx where x is a digit): kevin-morby")
	})

	t.Run("invalid key", func(t *testing.T) {
		t.Setenv(v1alpha1.PublicKeyEnvVar, "asdf")
		myBundle, err := testutil.GivenPackageBundle("../../api/testdata/bundle_one.yaml")
		require.Nil(t, err)

		err = myBundle.BundleValidate()

		assert.EqualError(t, err, "unable parse the public key (not PKIX)")
	})

	t.Run("empty env", func(t *testing.T) {
		t.Setenv(v1alpha1.PublicKeyEnvVar, "")
		myBundle, err := testutil.GivenPackageBundle("../../api/testdata/bundle_one.yaml")
		require.Nil(t, err)

		err = myBundle.BundleValidate()

		assert.EqualError(t, err, "The signature is invalid for the configured public key: MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEnP0Yo+ZxzPUEfohcG3bbJ8987UT4f0tj+XVBjS/s35wkfjrxTKrVZQpz3ta3zi5ZlgXzd7a20B1U1Py/TtPsxw==")
	})

	t.Run("env override", func(t *testing.T) {
		t.Setenv(v1alpha1.PublicKeyEnvVar, "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEvME/v61IfA4ulmgdF10Ae/WCRqtXvrUtF+0nu0dbdP36u3He4GRepYdQGCmbPe0463yAABZs01/Vv/v52ktlmg==")
		myBundle, err := testutil.GivenPackageBundle("../../api/testdata/bundle_one.yaml")
		require.Nil(t, err)

		err = myBundle.BundleValidate()

		assert.Nil(t, err)
	})
}
