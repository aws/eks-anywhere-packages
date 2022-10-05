package bundle

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	ctrlmocks "github.com/aws/eks-anywhere-packages/controllers/mocks"
)

const (
	testBundleRegistry = "public.ecr.aws/j0a1m4z9"
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
			Name:      "eksa-packages-cluster01",
			Namespace: api.PackageNamespace,
		},
		Spec: api.PackageBundleControllerSpec{
			ActiveBundle:         testBundleName,
			DefaultRegistry:      "public.ecr.aws/j0a1m4z9",
			DefaultImageRegistry: "783794618700.dkr.ecr.us-west-2.amazonaws.com",
			BundleRepository:     "eks-anywhere-packages-bundles",
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

		bundle, err := bundleClient.GetActiveBundle(ctx, "billy")

		assert.Equal(t, bundle.Name, testBundleName)
		assert.Equal(t, "hello-eks-anywhere", bundle.Spec.Packages[0].Name)
		assert.Equal(t, "public.ecr.aws/l0g8r8j6", bundle.Spec.Packages[0].Source.Registry)
		assert.NoError(t, err)
	})

	t.Run("no active bundle", func(t *testing.T) {
		pbc := givenPackageBundleController()
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		pbc.Spec.ActiveBundle = ""
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pbc)).SetArg(2, *pbc)

		bundle, err := bundleClient.GetActiveBundle(ctx, "billy")

		assert.Nil(t, bundle)
		assert.EqualError(t, err, "no activeBundle set in PackageBundleController")
	})

	t.Run("error path", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(fmt.Errorf("oops"))

		_, err := bundleClient.GetActiveBundle(ctx, "billy")

		assert.EqualError(t, err, "getting PackageBundleController: oops")
	})

	t.Run("other error path", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pbc)).SetArg(2, *pbc)
		mockClient.EXPECT().Get(ctx, gomock.Any(), gomock.Any()).Return(fmt.Errorf("oops"))

		_, err := bundleClient.GetActiveBundle(ctx, "billy")

		assert.EqualError(t, err, "oops")
	})
}

func doAndReturnBundleList(_ context.Context, bundles *api.PackageBundleList, _ *client.ListOptions) error {
	bundles.Items = []api.PackageBundle{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "v1-16-1003",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "v1-21-1002",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "v1-21-1001",
			},
		},
	}
	return nil
}

func doAndReturnBundle(_ context.Context, nn types.NamespacedName, theBundle *api.PackageBundle) error {
	theBundle.Name = nn.Name
	return nil
}

func TestBundleClient_GetBundle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	namedBundle := &api.PackageBundle{}
	namedBundle.Name = "v1-21-1003"
	key := types.NamespacedName{
		Name:      "v1-21-1003",
		Namespace: "eksa-packages",
	}

	t.Run("already exists", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		mockClient.EXPECT().Get(ctx, key, gomock.Any()).DoAndReturn(doAndReturnBundle)

		actualBundle, err := bundleClient.GetBundle(ctx, "v1-21-1003")

		assert.NotNil(t, actualBundle)
		assert.NoError(t, err)
	})

	t.Run("already exists error", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		mockClient.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(namedBundle)).Return(fmt.Errorf("boom"))

		actualBundle, err := bundleClient.GetBundle(ctx, "v1-21-1003")

		assert.Nil(t, actualBundle)
		assert.EqualError(t, err, "boom")
	})

	t.Run("returns nil when bundle does not", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		groupResource := schema.GroupResource{
			Group:    key.Name,
			Resource: "Namespace",
		}
		notFoundError := errors.NewNotFound(groupResource, key.Name)
		mockClient.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(namedBundle)).Return(notFoundError)

		actualBundle, err := bundleClient.GetBundle(ctx, "v1-21-1003")

		assert.Nil(t, actualBundle)
		assert.NoError(t, err)
	})
}

func TestBundleClient_GetBundleList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("golden path", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		mockClient.EXPECT().List(ctx, gomock.Any(), &client.ListOptions{Namespace: api.PackageNamespace}).DoAndReturn(doAndReturnBundleList)

		bundleItems, err := bundleClient.GetBundleList(ctx, "")

		assert.NoError(t, err)
		assert.Equal(t, "v1-21-1002", bundleItems[0].Name)
		assert.Equal(t, "v1-21-1001", bundleItems[1].Name)
		assert.Equal(t, "v1-16-1003", bundleItems[2].Name)
	})

	t.Run("error scenario", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		actualList := &api.PackageBundleList{}
		mockClient.EXPECT().List(ctx, actualList, &client.ListOptions{Namespace: api.PackageNamespace}).Return(fmt.Errorf("oops"))

		bundleItems, err := bundleClient.GetBundleList(ctx, "")

		assert.Nil(t, bundleItems)
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

		assert.NoError(t, err)
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

func TestBundleClient_CreateClusterNamespace(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := &v1.Namespace{}
	ns.Name = "eksa-packages-bobby"
	key := types.NamespacedName{
		Name: "eksa-packages-bobby",
	}

	t.Run("already exists", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		mockClient.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(ns)).Return(nil)

		err := bundleClient.CreateClusterNamespace(ctx, "bobby")

		assert.NoError(t, err)
	})

	t.Run("get error", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		mockClient.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(ns)).Return(fmt.Errorf("boom"))

		err := bundleClient.CreateClusterNamespace(ctx, "bobby")

		assert.EqualError(t, err, "boom")
	})

	t.Run("create namespace", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		groupResource := schema.GroupResource{
			Group:    key.Name,
			Resource: "Namespace",
		}
		notFoundError := errors.NewNotFound(groupResource, key.Name)
		mockClient.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(ns)).Return(notFoundError)
		mockClient.EXPECT().Create(ctx, ns).Return(nil)

		err := bundleClient.CreateClusterNamespace(ctx, "bobby")

		assert.NoError(t, err)
	})

	t.Run("create namespace error", func(t *testing.T) {
		mockClient := givenMockClient(t)
		bundleClient := NewPackageBundleClient(mockClient)
		groupResource := schema.GroupResource{
			Group:    key.Name,
			Resource: "Namespace",
		}
		notFoundError := errors.NewNotFound(groupResource, key.Name)
		mockClient.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(ns)).Return(notFoundError)
		mockClient.EXPECT().Create(ctx, ns).Return(fmt.Errorf("boom"))

		err := bundleClient.CreateClusterNamespace(ctx, "bobby")

		assert.EqualError(t, err, "boom")
	})
}
