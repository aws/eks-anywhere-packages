package bundle

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/testutil"
)

func TestDownloadBundle(t *testing.T) {
	t.Parallel()

	baseRef := "example.com/org"

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		puller := testutil.NewMockPuller()
		puller.WithFileData(t, "../../api/testdata/bundle_one.yaml")

		ecr := NewRegistryClient(puller)

		kubeVersion := "v1-21"
		tag := "latest"
		ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

		bundle, err := ecr.DownloadBundle(ctx, ref)

		assert.Nil(t, err)
		assert.NotNil(t, bundle)
		assert.Equal(t, 3, len(bundle.Spec.Packages))
		assert.Equal(t, "test", bundle.Spec.Packages[0].Name)
		assert.Equal(t, "flux", bundle.Spec.Packages[1].Name)
		assert.Equal(t, "harbor", bundle.Spec.Packages[2].Name)

	})

	t.Run("handles pull errors", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		puller := testutil.NewMockPuller()
		puller.WithError(fmt.Errorf("test error"))

		ecr := NewRegistryClient(puller)

		kubeVersion := "v1-21"
		tag := "latest"
		ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

		_, err := ecr.DownloadBundle(ctx, ref)
		assert.EqualError(t, err, "pulling package bundle: test error")
	})

	t.Run("errors on empty responses", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		puller := testutil.NewMockPuller()
		puller.WithData([]byte(""))

		ecr := NewRegistryClient(puller)

		kubeVersion := "v1-21"
		tag := "latest"
		ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

		_, err := ecr.DownloadBundle(ctx, ref)
		assert.EqualError(t, err, "package bundle artifact is empty")
	})

	t.Run("handles YAML unmarshalling errors", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		// stub oras.Pull
		puller := testutil.NewMockPuller()
		puller.WithData([]byte("invalid yaml"))

		ecr := NewRegistryClient(puller)

		kubeVersion := "v1-21"
		tag := "latest"
		ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

		_, err := ecr.DownloadBundle(ctx, ref)
		assert.EqualError(t, err, "unmarshalling package bundle: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type v1alpha1.PackageBundle")
	})
}

func TestBundleManager_LatestBundle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("latest bundle", func(t *testing.T) {
		puller := testutil.NewMockPuller()
		bundle := GivenBundle(api.PackageBundleStateAvailable)

		bundle.Namespace = "billy"
		bm := NewRegistryClient(puller)

		_, err := bm.LatestBundle(ctx, "test", "v1.21")

		assert.EqualError(t, err, "pulling package bundle: no mock data provided")
	})
}
