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
	"github.com/aws/eks-anywhere-packages/pkg/authenticator/mocks"
	bundleMocks "github.com/aws/eks-anywhere-packages/pkg/bundle/mocks"
)

const testPreviousBundleName = "v1-21-1002"
const testBundleName = "v1-21-1003"
const testNextBundleName = "v1-21-1004"
const testKubernetesVersion = "v1.21"

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

func givenBundleManager(t *testing.T) (*mocks.MockTargetClusterClient, *bundleMocks.MockRegistryClient, *bundleMocks.MockClient, *bundleManager) {
	tcc := mocks.NewMockTargetClusterClient(gomock.NewController(t))
	rc := bundleMocks.NewMockRegistryClient(gomock.NewController(t))
	bc := bundleMocks.NewMockClient(gomock.NewController(t))
	bm := NewBundleManager(logr.Discard(), rc, bc, tcc)
	return tcc, rc, bc, bm
}

func TestProcessBundle(t *testing.T) {
	ctx := context.Background()

	t.Run("ignore other namespaces", func(t *testing.T) {
		_, _, _, bm := givenBundleManager(t)
		bundle := GivenBundle("")
		bundle.Namespace = "billy"

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateIgnored, bundle.Status.State)
	})

	t.Run("already ignore other namespaces", func(t *testing.T) {
		_, _, _, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateIgnored)
		bundle.Namespace = "billy"

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.False(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateIgnored, bundle.Status.State)
	})

	t.Run("ignore invalid Kubernetes version", func(t *testing.T) {
		_, _, _, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateAvailable)
		bundle.Name = "v1-21-x"

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateInvalid, bundle.Status.State)
	})

	t.Run("marks state available", func(t *testing.T) {
		_, _, _, bm := givenBundleManager(t)
		bundle := GivenBundle("")

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateAvailable, bundle.Status.State)
	})

	t.Run("already marked state available", func(t *testing.T) {
		_, _, _, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateAvailable)
		bundle.Name = testPreviousBundleName

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.False(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateAvailable, bundle.Status.State)
	})
}

func TestBundleManager_ProcessBundleController(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	info := version.Info{
		GitVersion: testKubernetesVersion,
		Major:      "1",
		Minor:      "21",
	}
	allBundles := []api.PackageBundle{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: testBundleName,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "v1-21-1001",
			},
		},
	}

	t.Run("active to active", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(allBundles, nil)
		bc.EXPECT().CreateClusterNamespace(ctx, pbc.Name).Return(nil)
		bc.EXPECT().GetBundle(ctx, pbc.Spec.ActiveBundle).Return(&allBundles[0], nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateActive, pbc.Status.State)
	})

	t.Run("active missing active bundle", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(allBundles, nil)
		bc.EXPECT().CreateClusterNamespace(ctx, pbc.Name).Return(nil)
		bc.EXPECT().GetBundle(ctx, pbc.Spec.ActiveBundle).Return(nil, nil)
		rc.EXPECT().DownloadBundle(ctx, "public.ecr.aws/j0a1m4z9/eks-anywhere-packages-bundles:v1-21-1003").Return(&allBundles[0], nil)
		bc.EXPECT().CreateBundle(ctx, gomock.Any()).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateActive, pbc.Status.State)
	})

	t.Run("active missing active bundle download error", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(allBundles, nil)
		bc.EXPECT().CreateClusterNamespace(ctx, pbc.Name).Return(nil)
		bc.EXPECT().GetBundle(ctx, pbc.Spec.ActiveBundle).Return(nil, nil)
		rc.EXPECT().DownloadBundle(ctx, "public.ecr.aws/j0a1m4z9/eks-anywhere-packages-bundles:v1-21-1003").Return(&allBundles[0], fmt.Errorf("boom"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateActive, pbc.Status.State)
	})

	t.Run("active missing active bundle create error", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(allBundles, nil)
		bc.EXPECT().CreateClusterNamespace(ctx, pbc.Name).Return(nil)
		bc.EXPECT().GetBundle(ctx, pbc.Spec.ActiveBundle).Return(nil, nil)
		rc.EXPECT().DownloadBundle(ctx, "public.ecr.aws/j0a1m4z9/eks-anywhere-packages-bundles:v1-21-1003").Return(&allBundles[0], nil)
		bc.EXPECT().CreateBundle(ctx, gomock.Any()).Return(fmt.Errorf("boom"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateActive, pbc.Status.State)
	})

	t.Run("active to active get bundles error", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		assert.Equal(t, pbc.Spec.ActiveBundle, testBundleName)
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(nil, fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "getting bundle list: oops")
	})

	t.Run("active to disconnected", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		assert.Equal(t, pbc.Spec.ActiveBundle, testBundleName)
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, fmt.Errorf("ooops"))
		bc.EXPECT().SaveStatus(ctx, pbc).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateDisconnected, pbc.Status.State)
	})

	t.Run("disconnected to disconnected", func(t *testing.T) {
		tcc, rc, _, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateDisconnected
		assert.Equal(t, pbc.Spec.ActiveBundle, testBundleName)
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, fmt.Errorf("ooops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateDisconnected, pbc.Status.State)
	})

	t.Run("active to disconnected error", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		assert.Equal(t, pbc.Spec.ActiveBundle, testBundleName)
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, fmt.Errorf("ooops"))
		bc.EXPECT().SaveStatus(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating eksa-packages-cluster01 status to disconnected: oops")
	})

	t.Run("active to upgradeAvailable", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(allBundles, nil)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().CreateClusterNamespace(ctx, pbc.Name).Return(nil)
		bc.EXPECT().GetBundle(ctx, pbc.Spec.ActiveBundle).Return(&allBundles[0], nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateUpgradeAvailable, pbc.Status.State)
	})

	t.Run("active to ns error", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(allBundles, nil)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().CreateClusterNamespace(ctx, pbc.Name).Return(fmt.Errorf("boom"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "creating namespace for eksa-packages-cluster01: boom")
	})

	t.Run("active to upgradeAvailable active bundle error", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(allBundles, nil)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().CreateClusterNamespace(ctx, pbc.Name).Return(nil)
		bc.EXPECT().GetBundle(ctx, pbc.Spec.ActiveBundle).Return(nil, fmt.Errorf("boom"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
	})

	t.Run("active to upgradeAvailable error", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(allBundles, nil)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().CreateClusterNamespace(ctx, pbc.Name).Return(nil)
		bc.EXPECT().GetBundle(ctx, pbc.Spec.ActiveBundle).Return(&allBundles[0], nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating eksa-packages-cluster01 status to upgrade available: oops")
	})

	t.Run("active to upgradeAvailable create error", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(allBundles, nil)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "oops")
	})

	t.Run("upgradeAvailable to upgradeAvailable", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateUpgradeAvailable
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(allBundles, nil)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateUpgradeAvailable, pbc.Status.State)
	})

	t.Run("upgradeAvailable to active", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateUpgradeAvailable
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(allBundles, nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateActive, pbc.Status.State)
	})

	t.Run("upgradeAvailable to active error", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateUpgradeAvailable
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(allBundles, nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating eksa-packages-cluster01 status to active: oops")
	})

	t.Run("disconnected to active", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateDisconnected
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(allBundles, nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateActive, pbc.Status.State)
	})

	t.Run("disconnected to active error", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateDisconnected
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(allBundles, nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating eksa-packages-cluster01 status to active: oops")
	})

	t.Run("nothing to active bundle set", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = ""
		pbc.Spec.ActiveBundle = ""
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(allBundles, nil)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().Save(ctx, pbc).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateEnum(""), pbc.Status.State)
		assert.Equal(t, testNextBundleName, pbc.Spec.ActiveBundle)
	})

	t.Run("nothing to active bundle save error", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = ""
		pbc.Spec.ActiveBundle = ""
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(allBundles, nil)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().Save(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating eksa-packages-cluster01 activeBundle to v1-21-1004: oops")
	})

	t.Run("nothing to active state", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = ""
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(allBundles, nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateActive, pbc.Status.State)
		assert.Equal(t, testBundleName, pbc.Spec.ActiveBundle)
	})

	t.Run("nothing to active status save error", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = ""
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, "v1-21").Return(allBundles, nil)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating eksa-packages-cluster01 status to active: oops")
	})
}
