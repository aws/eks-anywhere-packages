package bundle

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	ctrlmocks "github.com/aws/eks-anywhere-packages/controllers/mocks"
)

const (
	testBundleRegistry   = "public.ecr.aws/j0a1m4z9"
	testBundleRepository = "eks-anywhere-package-bundles"
)

func givenMockClient(t *testing.T) *ctrlmocks.MockClient {
	goMockController := gomock.NewController(t)
	return ctrlmocks.NewMockClient(goMockController)
}

func givenBundle() *api.PackageBundle {
	return &api.PackageBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testBundleName,
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

func givenPackageBundleController() *api.PackageBundleController {
	return &api.PackageBundleController{
		ObjectMeta: metav1.ObjectMeta{
			Name:      api.PackageBundleControllerName,
			Namespace: api.PackageNamespace,
		},
		Spec: api.PackageBundleControllerSpec{
			ActiveBundle:         testBundleName,
			DefaultRegistry:      "public.ecr.aws/j0a1m4z9",
			DefaultImageRegistry: "783794618700.dkr.ecr.us-west-2.amazonaws.com",
			Source: api.PackageBundleControllerSource{
				Registry:   testBundleRegistry,
				Repository: testBundleRepository,
			},
		},
		Status: api.PackageBundleControllerStatus{
			State: api.BundleControllerStateActive,
		},
	}
}

func TestNewPackageBundleClient(t *testing.T) {
	t.Parallel()

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)

		assert.NotNil(t, bundleClient)
	})
}

func TestBundleClient_IsActive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pbc := givenPackageBundleController()

	t.Run("golden path returning true", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		bundle := GivenBundle(api.PackageBundleStateAvailable)
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pbc)).SetArg(2, *pbc)

		active, err := bundleClient.IsActive(ctx, bundle)

		assert.True(t, active)
		assert.Nil(t, err)
	})

	t.Run("error on failed get", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		bundle := GivenBundle(api.PackageBundleStateAvailable)
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pbc)).Return(fmt.Errorf("failed get"))

		_, err := bundleClient.IsActive(ctx, bundle)

		assert.NotNil(t, err)
	})

	t.Run("fails on wrong namespace", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		bundle := &api.PackageBundle{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "Wrong-Name",
			},
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateAvailable,
			},
		}
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pbc)).SetArg(2, *pbc)

		active, err := bundleClient.IsActive(ctx, bundle)

		assert.False(t, active)
		assert.Nil(t, err)
	})

	t.Run("fails on wrong name", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		bundle := &api.PackageBundle{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "non-empty",
				Namespace: api.PackageNamespace,
			},
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateAvailable,
			},
		}
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pbc)).SetArg(2, *pbc)

		active, err := bundleClient.IsActive(ctx, bundle)

		assert.False(t, active)
		assert.Nil(t, err)
	})
}

func TestBundleClient_GetActiveBundle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pbc := givenPackageBundleController()

	t.Run("golden path", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		testBundle := givenBundle()

		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pbc)).SetArg(2, *pbc)
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(testBundle)).SetArg(2, *testBundle)

		bundle, err := bundleClient.GetActiveBundle(ctx)

		assert.Equal(t, bundle.Name, testBundleName)
		assert.Equal(t, "hello-eks-anywhere", bundle.Spec.Packages[0].Name)
		assert.Equal(t, "public.ecr.aws/l0g8r8j6", bundle.Spec.Packages[0].Source.Registry)
		assert.Nil(t, err)
	})

	t.Run("no active bundle", func(t *testing.T) {
		pbc := givenPackageBundleController()
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		pbc.Spec.ActiveBundle = ""
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pbc)).SetArg(2, *pbc)

		bundle, err := bundleClient.GetActiveBundle(ctx)

		assert.Nil(t, bundle)
		assert.EqualError(t, err, "no activeBundle set in PackageBundleController")
	})

	t.Run("error path", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(fmt.Errorf("oops"))

		_, err := bundleClient.GetActiveBundle(ctx)

		assert.EqualError(t, err, "getting PackageBundleController: oops")
	})

	t.Run("other error path", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)

		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pbc)).SetArg(2, *pbc)
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(fmt.Errorf("oops"))

		_, err := bundleClient.GetActiveBundle(ctx)

		assert.EqualError(t, err, "oops")
	})
}

func TestBundleClient_GetActiveBundleNamespacedName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pbc := givenPackageBundleController()

	t.Run("golden path", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pbc))

		namespacedNames, err := bundleClient.GetActiveBundleNamespacedName(ctx)

		assert.Equal(t, api.PackageNamespace, namespacedNames.Namespace)
		assert.Equal(t, "", namespacedNames.Name)
		assert.Nil(t, err)
	})

	t.Run("error path", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(fmt.Errorf("oops"))

		namespacedNames, err := bundleClient.GetActiveBundleNamespacedName(ctx)

		assert.Equal(t, "", namespacedNames.Namespace)
		assert.Equal(t, "", namespacedNames.Name)
		assert.EqualError(t, err, "getting PackageBundleController: oops")
	})
}

func TestBundleClient_GetBundleList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("golden path", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		actualList := &api.PackageBundleList{}
		mockClient.EXPECT().List(ctx, actualList, &client.ListOptions{Namespace: api.PackageNamespace}).Return(nil)

		err := bundleClient.GetBundleList(ctx, actualList)

		assert.Nil(t, err)
	})

	t.Run("error scenario", func(t *testing.T) {
		mockClient := givenMockClient(t)
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
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		actualBundle := &api.PackageBundle{}
		mockClient.EXPECT().Create(ctx, actualBundle).Return(nil)

		err := bundleClient.CreateBundle(ctx, actualBundle)

		assert.Nil(t, err)
	})

	t.Run("error scenario", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		actualBundle := &api.PackageBundle{}
		mockClient.EXPECT().Create(ctx, actualBundle).Return(fmt.Errorf("oops"))

		err := bundleClient.CreateBundle(ctx, actualBundle)

		assert.EqualError(t, err, "creating new package bundle: oops")
	})
}
