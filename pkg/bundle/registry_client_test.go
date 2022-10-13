package bundle

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/aws/eks-anywhere-packages/pkg/artifacts/mocks"
)

func TestDownloadBundle(t *testing.T) {
	t.Parallel()

	baseRef := "example.com/org"

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		puller := mocks.NewMockPuller(gomock.NewController(t))
		contents, err := os.ReadFile("../../api/testdata/bundle_one.yaml")
		assert.NoError(t, err)
		puller.EXPECT().Pull(ctx, "example.com/org:v1-21-latest").Return(contents, nil)
		ecr := NewRegistryClient(puller)

		kubeVersion := "v1-21"
		tag := "latest"
		ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

		bundle, err := ecr.DownloadBundle(ctx, ref)

		assert.NoError(t, err)
		assert.NotNil(t, bundle)
		assert.Equal(t, 3, len(bundle.Spec.Packages))
		assert.Equal(t, "hello-eks-anywhere", bundle.Spec.Packages[0].Name)
		assert.Equal(t, "flux", bundle.Spec.Packages[1].Name)
		assert.Equal(t, "harbor", bundle.Spec.Packages[2].Name)

	})

	t.Run("handles pull errors", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		puller := mocks.NewMockPuller(gomock.NewController(t))
		puller.EXPECT().Pull(ctx, "example.com/org:v1-21-latest").Return(nil, fmt.Errorf("test error"))

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
		puller := mocks.NewMockPuller(gomock.NewController(t))
		puller.EXPECT().Pull(ctx, "example.com/org:v1-21-latest").Return([]byte(""), nil)

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
		puller := mocks.NewMockPuller(gomock.NewController(t))
		puller.EXPECT().Pull(ctx, "example.com/org:v1-21-latest").Return([]byte("invalid yaml"), nil)

		ecr := NewRegistryClient(puller)

		kubeVersion := "v1-21"
		tag := "latest"
		ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

		_, err := ecr.DownloadBundle(ctx, ref)
		assert.EqualError(t, err, "unmarshalling package bundle: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type v1alpha1.PackageBundle")
	})
}

func TestRegistryClient_LatestBundle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("latest bundle error", func(t *testing.T) {
		puller := mocks.NewMockPuller(gomock.NewController(t))
		puller.EXPECT().Pull(ctx, "test:v1-21-latest").Return(nil, fmt.Errorf("oops"))
		bm := NewRegistryClient(puller)

		result, err := bm.LatestBundle(ctx, "test", "1", "21")

		assert.EqualError(t, err, "pulling package bundle: oops")
		assert.Nil(t, result)
	})
}
