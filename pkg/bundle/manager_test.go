package bundle

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	bundleMocks "github.com/aws/eks-anywhere-packages/pkg/bundle/mocks"
)

const testPreviousBundleName = "v1-21-1002"
const testBundleName = "v1-21-1003"
const testNextBundleName = "v1-21-1004"
const testKubernetesVersion = "v1-21"

func GivenBundle(state api.PackageBundleStateEnum) *api.PackageBundle {
	return &api.PackageBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testBundleName,
			Namespace: api.PackageNamespace,
		},
		Status: api.PackageBundleStatus{
			State: state,
		},
	}
}

func mockGetBundleListNone(_ context.Context, bundles *api.PackageBundleList) error {
	bundles.Items = []api.PackageBundle{}
	return nil
}

func mockGetBundleList(_ context.Context, bundles *api.PackageBundleList) error {
	bundles.Items = []api.PackageBundle{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testBundleName,
				Namespace: "eksa-packages",
			},
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateInactive,
			},
		},
	}
	return nil
}

func givenBundleManager(t *testing.T) (version.Info, *bundleMocks.MockRegistryClient, *bundleMocks.MockClient, *bundleManager) {
	rc := bundleMocks.NewMockRegistryClient(gomock.NewController(t))
	bc := bundleMocks.NewMockClient(gomock.NewController(t))
	info := version.Info{Major: "1", Minor: "21+"}
	bm := NewBundleManager(logr.Discard(), info, rc, bc)
	return info, rc, bc, bm
}

func TestProcessBundle(t *testing.T) {
	ctx := context.Background()

	t.Run("isActive error", func(t *testing.T) {
		_, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateInactive)
		bc.EXPECT().IsActive(ctx, bundle).Return(false, fmt.Errorf("oops"))

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.False(t, update)
		assert.EqualError(t, err, "getting active bundle: oops")
	})

	t.Run("ignore other namespaces", func(t *testing.T) {
		_, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateInactive)
		bundle.Namespace = "billy"
		bundle.Name = testNextBundleName
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateIgnored, bundle.Status.State)
	})

	t.Run("ignore incompatible Kubernetes version", func(t *testing.T) {
		_, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateInactive)
		bundle.Name = "v1-01-1"
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateIgnoredVersion, bundle.Status.State)
	})

	t.Run("already ignored incompatible Kubernetes version", func(t *testing.T) {
		_, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateInactive)
		bundle.Name = "v1-01-1"
		bundle.Status.State = api.PackageBundleStateIgnoredVersion
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.False(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateIgnoredVersion, bundle.Status.State)
	})

	t.Run("ignore invalid Kubernetes version", func(t *testing.T) {
		_, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateInactive)
		bundle.Name = "v1-21-x"
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateInvalidVersion, bundle.Status.State)
	})

	t.Run("ignored is ignored", func(t *testing.T) {
		_, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateIgnored)
		bundle.Namespace = "billy"
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.False(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateIgnored, bundle.Status.State)
	})

	t.Run("marks state active", func(t *testing.T) {
		_, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateInactive)
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleListNone)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateActive, bundle.Status.State)
	})

	t.Run("marks state inactive", func(t *testing.T) {
		_, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateActive)
		bundle.Name = testPreviousBundleName
		bc.EXPECT().IsActive(ctx, bundle).Return(false, nil)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateInactive, bundle.Status.State)
	})

	t.Run("leaves state as-is (inactive)", func(t *testing.T) {
		_, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateInactive)
		bundle.Name = testPreviousBundleName
		bc.EXPECT().IsActive(ctx, bundle).Return(false, nil)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.False(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateInactive, bundle.Status.State)
	})

	t.Run("leaves state as-is (active) empty list", func(t *testing.T) {
		_, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateActive)
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleListNone)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.False(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateActive, bundle.Status.State)
	})

	t.Run("leaves state as-is (active)", func(t *testing.T) {
		_, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateActive)
		bundle.Status.State = api.PackageBundleStateActive
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.False(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateActive, bundle.Status.State)
	})

	t.Run("marks state upgrade available", func(t *testing.T) {
		_, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateActive)
		bundle.Name = testNextBundleName
		bundle.Status.State = api.PackageBundleStateActive
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateUpgradeAvailable, bundle.Status.State)
	})

	t.Run("leaves state as-is (upgrade available)", func(t *testing.T) {
		_, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateUpgradeAvailable)
		bundle.Name = testNextBundleName
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.False(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateUpgradeAvailable, bundle.Status.State)
	})

	t.Run("get bundle list error", func(t *testing.T) {
		_, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateActive)
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).Return(fmt.Errorf("oops"))

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.False(t, update)
		assert.EqualError(t, err, "getting bundle list: oops")
	})
}

func TestSortBundleNewestFirst(t *testing.T) {
	t.Parallel()

	t.Run("it sorts newest version first", func(t *testing.T) {
		_, _, _, bm := givenBundleManager(t)
		allBundles := []api.PackageBundle{
			*api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_one.yaml"),
			*api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_two.yaml"),
		}

		bm.SortBundlesDescending(allBundles)
		if assert.Greater(t, len(allBundles), 1) {
			assert.Equal(t, testPreviousBundleName, allBundles[0].Name)
			assert.Equal(t, "v1-21-1001", allBundles[1].Name)
		}
	})

	t.Run("invalid names go to the end", func(t *testing.T) {
		_, _, _, bm := givenBundleManager(t)
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

		bm.SortBundlesDescending(allBundles)
		if assert.Greater(t, len(allBundles), 2) {
			assert.Equal(t, testPreviousBundleName, allBundles[0].Name)
			assert.Equal(t, "v1-21-1001", allBundles[1].Name)
			assert.Equal(t, "v1-16-1003", allBundles[2].Name)

		}
	})
}

func TestBundleManager_ProcessBundleController(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("active to active", func(t *testing.T) {
		_, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		assert.Equal(t, pbc.Spec.ActiveBundle, testBundleName)
		latestBundle := givenBundle()
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-package-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.Nil(t, err)
		assert.Equal(t, api.BundleControllerStateActive, pbc.Status.State)
	})

	t.Run("active to active get bundles error", func(t *testing.T) {
		_, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		assert.Equal(t, pbc.Spec.ActiveBundle, testBundleName)
		latestBundle := givenBundle()
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-package-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "getting bundle list: oops")
	})

	t.Run("active to disconnected", func(t *testing.T) {
		_, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		assert.Equal(t, pbc.Spec.ActiveBundle, testBundleName)
		latestBundle := givenBundle()
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-package-bundles", testKubernetesVersion).Return(latestBundle, fmt.Errorf("ooops"))
		bc.EXPECT().SaveStatus(ctx, pbc).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.Nil(t, err)
		assert.Equal(t, api.BundleControllerStateDisconnected, pbc.Status.State)
	})

	t.Run("disconnected to disconnected", func(t *testing.T) {
		_, rc, _, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateDisconnected
		assert.Equal(t, pbc.Spec.ActiveBundle, testBundleName)
		latestBundle := givenBundle()
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-package-bundles", testKubernetesVersion).Return(latestBundle, fmt.Errorf("ooops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.Nil(t, err)
		assert.Equal(t, api.BundleControllerStateDisconnected, pbc.Status.State)
	})

	t.Run("active to disconnected error", func(t *testing.T) {
		_, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		assert.Equal(t, pbc.Spec.ActiveBundle, testBundleName)
		latestBundle := givenBundle()
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-package-bundles", testKubernetesVersion).Return(latestBundle, fmt.Errorf("ooops"))
		bc.EXPECT().SaveStatus(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating eksa-packages-bundle-controller status to disconnected: oops")
	})

	t.Run("active to upgradeAvailable", func(t *testing.T) {
		_, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-package-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.Nil(t, err)
		assert.Equal(t, api.BundleControllerStateUpgradeAvailable, pbc.Status.State)
	})

	t.Run("active to upgradeAvailable error", func(t *testing.T) {
		_, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-package-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating eksa-packages-bundle-controller status to upgrade available: oops")
	})

	t.Run("active to upgradeAvailable create error", func(t *testing.T) {
		_, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-package-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "creating new package bundle: oops")
	})

	t.Run("upgradeAvailable to upgradeAvailable", func(t *testing.T) {
		_, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateUpgradeAvailable
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-package-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.Nil(t, err)
		assert.Equal(t, api.BundleControllerStateUpgradeAvailable, pbc.Status.State)
	})

	t.Run("upgradeAvailable to active", func(t *testing.T) {
		_, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateUpgradeAvailable
		latestBundle := givenBundle()
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-package-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.Nil(t, err)
		assert.Equal(t, api.BundleControllerStateActive, pbc.Status.State)
	})

	t.Run("upgradeAvailable to active error", func(t *testing.T) {
		_, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateUpgradeAvailable
		latestBundle := givenBundle()
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-package-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating eksa-packages-bundle-controller status to active: oops")
	})

	t.Run("disconnected to active", func(t *testing.T) {
		_, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateDisconnected
		latestBundle := givenBundle()
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-package-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.Nil(t, err)
		assert.Equal(t, api.BundleControllerStateActive, pbc.Status.State)
	})

	t.Run("disconnected to active error", func(t *testing.T) {
		_, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateDisconnected
		latestBundle := givenBundle()
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-package-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating eksa-packages-bundle-controller status to active: oops")
	})

	t.Run("nothing to active bundle set", func(t *testing.T) {
		_, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = ""
		pbc.Spec.ActiveBundle = ""
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-package-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().Save(ctx, pbc).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.Nil(t, err)
		assert.Equal(t, api.BundleControllerStateEnum(""), pbc.Status.State)
		assert.Equal(t, testNextBundleName, pbc.Spec.ActiveBundle)
	})

	t.Run("nothing to active bundle save error", func(t *testing.T) {
		_, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = ""
		pbc.Spec.ActiveBundle = ""
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-package-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().Save(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating eksa-packages-bundle-controller activeBundle to v1-21-1004: oops")
	})

	t.Run("nothing to active state", func(t *testing.T) {
		_, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = ""
		latestBundle := givenBundle()
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-package-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.Nil(t, err)
		assert.Equal(t, api.BundleControllerStateActive, pbc.Status.State)
		assert.Equal(t, testBundleName, pbc.Spec.ActiveBundle)
	})

	t.Run("nothing to active status save error", func(t *testing.T) {
		_, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = ""
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-package-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating eksa-packages-bundle-controller status to active: oops")
	})
}
