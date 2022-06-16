package bundle

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	bundleMocks "github.com/aws/eks-anywhere-packages/pkg/bundle/mocks"
	"github.com/aws/eks-anywhere-packages/pkg/testutil"
)

func TestDownloadBundle(t *testing.T) {
	t.Parallel()

	baseRef := "example.com/org"
	discovery := testutil.NewFakeDiscoveryWithDefaults()

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		puller := testutil.NewMockPuller()
		puller.WithFileData(t, "../../api/testdata/bundle_one.yaml")

		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		kubeVersion := "v1-21"
		tag := "latest"
		ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

		bundle, err := bm.DownloadBundle(ctx, ref)

		if err != nil {
			t.Fatalf("expected no error, got: %s", err)
		}

		if bundle == nil {
			t.Errorf("expected bundle to be non-nil")
		}

		if bundle != nil && len(bundle.Spec.Packages) != 3 {
			t.Errorf("expected three packages to be defined, found %d",
				len(bundle.Spec.Packages))
		}
		if bundle.Spec.Packages[0].Name != "test" {
			t.Errorf("expected first package name to be \"test\", got: %q",
				bundle.Spec.Packages[0].Name)
		}
		if bundle.Spec.Packages[1].Name != "flux" {
			t.Errorf("expected second package name to be \"flux\", got: %q",
				bundle.Spec.Packages[1].Name)
		}
		if bundle.Spec.Packages[2].Name != "harbor" {
			t.Errorf("expected third package name to be \"harbor\", got: %q",
				bundle.Spec.Packages[2].Name)
		}
	})

	t.Run("handles pull errors", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		puller := testutil.NewMockPuller()
		puller.WithError(fmt.Errorf("test error"))

		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		kubeVersion := "v1-21"
		tag := "latest"
		ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

		_, err := bm.DownloadBundle(ctx, ref)
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})

	t.Run("errors on empty repsonses", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		puller := testutil.NewMockPuller()
		puller.WithData([]byte(""))

		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		kubeVersion := "v1-21"
		tag := "latest"
		ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

		_, err := bm.DownloadBundle(ctx, ref)
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})

	t.Run("handles YAML unmarshaling errors", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		// stub oras.Pull
		puller := testutil.NewMockPuller()
		puller.WithData([]byte("invalid yaml"))

		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		kubeVersion := "v1-21"
		tag := "latest"
		ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

		_, err := bm.DownloadBundle(ctx, ref)
		if err == nil {
			t.Errorf("expected error, got nil")
		}
		// The k8s YAML library converts everything to JSON, so the error we'll
		// get will be a JSON one.
		if !strings.Contains(err.Error(), "JSON") {
			t.Errorf("expected YAML-related error, got: %s", err)
		}
	})
}

func TestKubeVersion(t *testing.T) {
	t.Parallel()

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		expected := "v1.21"
		if ver, _ := kubeVersion("v1.21-42"); ver != expected {
			t.Errorf("expected %q, got %q", expected, ver)
		}
	})

	t.Run("error on blank version", func(t *testing.T) {
		t.Parallel()

		_, err := kubeVersion("")
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})
}

func TestPackageVersion(t *testing.T) {
	t.Parallel()

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		got, err := bm.apiVersion()
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		expected := "v1-21"
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("minor version+", func(t *testing.T) {
		t.Parallel()

		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		got, err := bm.apiVersion()
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		expected := "v1-21"
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		got, err := bm.apiVersion()
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		expected := "v1-21"
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})
}

func givenPackageBundle(state api.PackageBundleStateEnum) *api.PackageBundle {
	return &api.PackageBundle{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: api.PackageNamespace,
		},
		Status: api.PackageBundleStatus{
			State: state,
		},
	}
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("ignore other namespaces", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := givenPackageBundle(api.PackageBundleStateInactive)
		bundle.Namespace = "billy"
		bundle.Name = "v1-21"
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		mockBundleClient.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		update, err := bm.Update(ctx, bundle)
		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateIgnored, bundle.Status.State)
	})

	t.Run("ignore incompatible Kubernetes version", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := givenPackageBundle(api.PackageBundleStateInactive)
		bundle.Name = "v1-01"
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		mockBundleClient.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		update, err := bm.Update(ctx, bundle)
		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateIgnored, bundle.Status.State)
	})

	t.Run("ignored is ignored", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := givenPackageBundle(api.PackageBundleStateIgnored)
		bundle.Namespace = "billy"
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		mockBundleClient.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		update, err := bm.Update(ctx, bundle)
		assert.False(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateIgnored, bundle.Status.State)
	})

	t.Run("marks state active", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := givenPackageBundle(api.PackageBundleStateInactive)
		bundle.Name = "v1-21"
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		mockBundleClient.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		mockBundleClient.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleListNone)
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		update, err := bm.Update(ctx, bundle)
		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateActive, bundle.Status.State)
	})

	t.Run("marks state inactive", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := givenPackageBundle(api.PackageBundleStateActive)
		bundle.Name = "v1-21"
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		mockBundleClient.EXPECT().IsActive(ctx, bundle).Return(false, nil)
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		update, err := bm.Update(ctx, bundle)
		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateInactive, bundle.Status.State)
	})

	t.Run("leaves state as-is (inactive)", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := givenPackageBundle(api.PackageBundleStateInactive)
		bundle.Name = "v1-21"
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		mockBundleClient.EXPECT().IsActive(ctx, bundle).Return(false, nil)
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		update, err := bm.Update(ctx, bundle)
		assert.False(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateInactive, bundle.Status.State)
	})

	t.Run("leaves state as-is (active) empty list", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := givenPackageBundle(api.PackageBundleStateActive)
		bundle.Name = "v1-21-1003"
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		mockBundleClient.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		mockBundleClient.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleListNone)
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		update, err := bm.Update(ctx, bundle)
		assert.False(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateActive, bundle.Status.State)
	})

	t.Run("leaves state as-is (active)", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := givenPackageBundle(api.PackageBundleStateActive)
		bundle.Name = "v1-21-1003"
		bundle.Status.State = api.PackageBundleStateActive
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		mockBundleClient.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		mockBundleClient.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		update, err := bm.Update(ctx, bundle)
		assert.False(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateActive, bundle.Status.State)
	})

	t.Run("marks state upgrade available", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := givenPackageBundle(api.PackageBundleStateActive)
		bundle.Name = "v1-21-1004"
		bundle.Status.State = api.PackageBundleStateActive
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		mockBundleClient.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		mockBundleClient.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		update, err := bm.Update(ctx, bundle)
		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateUpgradeAvailable, bundle.Status.State)
	})

	t.Run("leaves state as-is (upgrade available)", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := givenPackageBundle(api.PackageBundleStateActive)
		bundle.Name = "v1-21-1001"
		bundle.Status.State = api.PackageBundleStateUpgradeAvailable
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		mockBundleClient.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		mockBundleClient.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		update, err := bm.Update(ctx, bundle)
		assert.False(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateUpgradeAvailable, bundle.Status.State)
	})

	t.Run("get bundle list error", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := givenPackageBundle(api.PackageBundleStateActive)
		bundle.Name = "v1-21-1003"
		bundle.Status.State = api.PackageBundleStateActive
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		mockBundleClient.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		mockBundleClient.EXPECT().GetBundleList(ctx, gomock.Any()).Return(fmt.Errorf("oops"))
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		update, err := bm.Update(ctx, bundle)
		assert.False(t, update)
		assert.EqualError(t, err, "getting bundle list: oops")
	})
}

func TestSortBundleNewestFirst(t *testing.T) {
	t.Run("it sorts newest version first", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		allBundles := []api.PackageBundle{
			*api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_one.yaml"),
			*api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_two.yaml"),
		}

		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)
		bm.SortBundlesDescending(allBundles)
		if assert.Greater(t, len(allBundles), 1) {
			assert.Equal(t, "v1-21-1002", allBundles[0].Name)
			assert.Equal(t, "v1-21-1001", allBundles[1].Name)
		}
	})

	t.Run("invalid names go to the end", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		allBundles := []api.PackageBundle{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "v1-16-1003",
				},
				Status: api.PackageBundleStatus{
					State: api.PackageBundleStateInactive,
				},
			},
			*api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_one.yaml"),
			*api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_two.yaml"),
		}

		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)
		bm.SortBundlesDescending(allBundles)
		if assert.Greater(t, len(allBundles), 2) {
			assert.Equal(t, "v1-21-1002", allBundles[0].Name)
			assert.Equal(t, "v1-21-1001", allBundles[1].Name)
			assert.Equal(t, "v1-16-1003", allBundles[2].Name)

		}
	})
}

func mockGetBundleListNone(_ context.Context, bundles *api.PackageBundleList) error {
	bundles.Items = []api.PackageBundle{}
	return nil
}

func mockGetBundleList(_ context.Context, bundles *api.PackageBundleList) error {
	bundles.Items = []api.PackageBundle{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "v1-21-1003",
				Namespace: "eksa-packages",
			},
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateInactive,
			},
		},
	}
	return nil
}

func TestBundleManager_UpdateLatestBundle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("unknown bundle", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := givenPackageBundle(api.PackageBundleStateInactive)
		bundle.Namespace = "eksa-packages"
		bundle.Name = "v1-21-1004"
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)
		mockBundleClient.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		mockBundleClient.EXPECT().CreateBundle(ctx, bundle).Return(nil)

		err := bm.ProcessLatestBundle(ctx, bundle)

		assert.Nil(t, err)
	})

	t.Run("known bundle", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := givenPackageBundle(api.PackageBundleStateInactive)
		bundle.Namespace = "eksa-packages"
		bundle.Name = "v1-21-1003"
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		mockBundleClient.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		err := bm.ProcessLatestBundle(ctx, bundle)

		assert.Nil(t, err)
	})

	t.Run("bundle create error", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := givenPackageBundle(api.PackageBundleStateInactive)
		bundle.Namespace = "eksa-packages"
		bundle.Name = "v1-21-1004"
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)
		mockBundleClient.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		mockBundleClient.EXPECT().CreateBundle(ctx, bundle).Return(fmt.Errorf("oops"))

		err := bm.ProcessLatestBundle(ctx, bundle)

		assert.EqualError(t, err, "creating new package bundle: oops")
	})

	t.Run("bundle list error", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := givenPackageBundle(api.PackageBundleStateInactive)
		bundle.Namespace = "eksa-packages"
		bundle.Name = "v1-21-1003"
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		mockBundleClient.EXPECT().GetBundleList(ctx, gomock.Any()).Return(fmt.Errorf("oops"))
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		err := bm.ProcessLatestBundle(ctx, bundle)

		assert.EqualError(t, err, "getting bundle list: oops")
	})
}

func TestBundleManager_LatestBundle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("latest bundle", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := givenPackageBundle(api.PackageBundleStateInactive)

		bundle.Namespace = "billy"
		bundle.Name = "v1-21-1003"
		mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
		bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

		_, err := bm.LatestBundle(ctx, "test")

		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})
}
