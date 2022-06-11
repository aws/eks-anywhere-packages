package bundle

import (
	"context"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	ctrlmocks "github.com/aws/eks-anywhere-packages/controllers/mocks"
)

func TestNewPackageBundleClient(t *testing.T) {
	t.Parallel()

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		mockClient := newMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)

		assert.NotNil(t, bundleClient)
	})
}

func TestBundleClient_IsActive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pbc := newMockPackageBundleController()

	t.Run("golden path returning true", func(t *testing.T) {
		mockClient := newMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		bundle := givenPackageBundle(api.PackageBundleStateInactive)
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(&pbc))

		active, err := bundleClient.IsActive(ctx, bundle)

		assert.True(t, active)
		assert.Nil(t, err)
	})

	t.Run("error on failed get", func(t *testing.T) {
		mockClient := newMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		bundle := givenPackageBundle(api.PackageBundleStateInactive)
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(&pbc)).Return(fmt.Errorf("failed get"))

		_, err := bundleClient.IsActive(ctx, bundle)

		assert.NotNil(t, err)
	})

	t.Run("fails on wrong namespace", func(t *testing.T) {
		mockClient := newMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		bundle := &api.PackageBundle{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "Wrong-Name",
			},
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateActive,
			},
		}
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(&pbc))

		active, err := bundleClient.IsActive(ctx, bundle)

		assert.False(t, active)
		assert.Nil(t, err)
	})

	t.Run("fails on wrong name", func(t *testing.T) {
		mockClient := newMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		bundle := &api.PackageBundle{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "non-empty",
				Namespace: api.PackageNamespace,
			},
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateActive,
			},
		}
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(&pbc))

		active, err := bundleClient.IsActive(ctx, bundle)

		assert.False(t, active)
		assert.Nil(t, err)
	})
}

func TestBundleClient_GetActiveBundle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pbc := newMockPackageBundleController()

	t.Run("golden path", func(t *testing.T) {
		mockClient := newMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		mockBundle := newMockBundle()

		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(&pbc))
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(&mockBundle)).SetArg(2, mockBundle)

		bundle, err := bundleClient.GetActiveBundle(ctx)

		assert.Equal(t, bundle.Name, "test-name")
		assert.Nil(t, err)
	})
}

func TestBundleClient_GetActiveBundleNamespacedName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pbc := newMockPackageBundleController()

	t.Run("golden path", func(t *testing.T) {
		mockClient := newMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(&pbc))

		namespacedNames, err := bundleClient.GetActiveBundleNamespacedName(ctx)

		assert.Equal(t, api.PackageNamespace, namespacedNames.Namespace)
		assert.Equal(t, "", namespacedNames.Name)
		assert.Nil(t, err)
	})
}

func TestBundleClient_GetBundleList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("golden path", func(t *testing.T) {
		mockClient := newMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		actualList := &api.PackageBundleList{}
		mockClient.EXPECT().List(ctx, actualList, &client.ListOptions{Namespace: api.PackageNamespace}).Return(nil)

		err := bundleClient.GetBundleList(ctx, actualList)

		assert.Nil(t, err)
	})

	t.Run("error scenario", func(t *testing.T) {
		mockClient := newMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		actualList := &api.PackageBundleList{}
		mockClient.EXPECT().List(ctx, actualList, &client.ListOptions{Namespace: api.PackageNamespace}).Return(fmt.Errorf("oops"))

		err := bundleClient.GetBundleList(ctx, actualList)

		assert.EqualError(t, err, "listing package bundles: oops")
	})
}

func TestBundleClient_CreateBundle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("golden path", func(t *testing.T) {
		mockClient := newMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		actualBundle := &api.PackageBundle{}
		mockClient.EXPECT().Create(ctx, actualBundle).Return(nil)

		err := bundleClient.CreateBundle(ctx, actualBundle)

		assert.Nil(t, err)
	})

	t.Run("error scenario", func(t *testing.T) {
		mockClient := newMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		actualBundle := &api.PackageBundle{}
		mockClient.EXPECT().Create(ctx, actualBundle).Return(fmt.Errorf("oops"))

		err := bundleClient.CreateBundle(ctx, actualBundle)

		assert.EqualError(t, err, "creating new package bundle: oops")
	})
}

// Helpers
func newMockClient(t *testing.T) *ctrlmocks.MockClient {
	goMockController := gomock.NewController(t)
	return ctrlmocks.NewMockClient(goMockController)
}

func newMockBundle() api.PackageBundle {
	return api.PackageBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: api.PackageNamespace,
		},
		Spec: api.PackageBundleSpec{
			Packages: []api.BundlePackage{
				{
					Name: "hello-eks-anywhere",
					Source: api.BundlePackageSource{
						Registry:   "public.ecr.aws/l0g8r8j6",
						Repository: "hello-eks-anywhere",
						Versions: []api.SourceVersion{
							{Name: "0.1.1", Digest: "sha256:deadbeef"},
							{Name: "0.1.0", Digest: "sha256:cafebabe"},
						},
					},
				},
			},
		},
	}
}

func newMockPackageBundleController() api.PackageBundleController {
	return api.PackageBundleController{}
}
