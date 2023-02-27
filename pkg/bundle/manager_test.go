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
	"k8s.io/client-go/rest"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/authenticator/mocks"
	bundleMocks "github.com/aws/eks-anywhere-packages/pkg/bundle/mocks"
	"github.com/aws/eks-anywhere-packages/pkg/config"
)

const (
	testPreviousBundleName = "v1-21-1002"
	testBundleName         = "v1-21-1003"
	testNextBundleName     = "v1-21-1004"
	testKubeMajor          = "1"
	testKubeMinor          = "21"
	testKubernetesVersion  = "v1.21"
)

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
	cfg := config.GetConfig()
	cfg.BuildInfo.Version = "v2.2.2"
	bm := NewBundleManager(logr.Discard(), rc, bc, tcc, cfg)
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

	t.Run("newer controller version required", func(t *testing.T) {
		_, _, _, bm := givenBundleManager(t)
		bundle := GivenBundle("")
		bundle.Spec.MinVersion = "v4.4.4"
		bundle.Name = testPreviousBundleName

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateUpgradeRequired, bundle.Status.State)

		// No change results in no update needed
		update, err = bm.ProcessBundle(ctx, bundle)

		assert.False(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateUpgradeRequired, bundle.Status.State)

		// Bundle becomes available after upgrade
		bm.config.BuildInfo.Version = "v4.4.5"
		update, err = bm.ProcessBundle(ctx, bundle)

		assert.True(t, update)
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
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
		bc.EXPECT().CreateClusterConfigMap(ctx, pbc.Name).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateActive, pbc.Status.State)
	})

	t.Run("active missing active bundle", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Spec.ActiveBundle = "v1-21-1002"
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
		bc.EXPECT().CreateClusterConfigMap(ctx, pbc.Name).Return(nil)
		rc.EXPECT().DownloadBundle(ctx, "public.ecr.aws/l0g8r8j6/eks-anywhere-packages-bundles:v1-21-1002", pbc.Name).Return(&allBundles[0], nil)
		bc.EXPECT().CreateBundle(ctx, gomock.Any()).Return(nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(nil) // update available

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateUpgradeAvailable, pbc.Status.State)
	})

	t.Run("active missing active bundle download error", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Spec.ActiveBundle = "v1-21-1002"
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
		bc.EXPECT().CreateClusterConfigMap(ctx, pbc.Name).Return(nil)
		rc.EXPECT().DownloadBundle(ctx, "public.ecr.aws/l0g8r8j6/eks-anywhere-packages-bundles:v1-21-1002", pbc.Name).Return(&allBundles[0], fmt.Errorf("boom"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateActive, pbc.Status.State)
	})

	t.Run("active missing active bundle create error", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Spec.ActiveBundle = "v1-21-1002"
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
		bc.EXPECT().CreateClusterConfigMap(ctx, pbc.Name).Return(nil)
		rc.EXPECT().DownloadBundle(ctx, "public.ecr.aws/l0g8r8j6/eks-anywhere-packages-bundles:v1-21-1002", pbc.Name).Return(&allBundles[0], nil)
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
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(nil, fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "getting bundle list: oops")
	})

	t.Run("active to disconnected", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		assert.Equal(t, pbc.Spec.ActiveBundle, testBundleName)
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, fmt.Errorf("ooops"))
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
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, fmt.Errorf("ooops"))

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
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, fmt.Errorf("ooops"))
		bc.EXPECT().SaveStatus(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating cluster01 status to disconnected: oops")
	})

	t.Run("active to upgradeAvailable", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().CreateClusterConfigMap(ctx, pbc.Name).Return(nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateUpgradeAvailable, pbc.Status.State)
	})

	t.Run("active to cm error", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().CreateClusterConfigMap(ctx, pbc.Name).Return(fmt.Errorf("boom"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "creating configmap for cluster01: boom")
	})

	t.Run("active to upgradeAvailable error", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().CreateClusterConfigMap(ctx, pbc.Name).Return(nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating cluster01 status to upgrade available: oops")
	})

	t.Run("active to upgradeAvailable create error", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "oops")
	})

	t.Run("upgradeAvailable to upgradeAvailable", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateUpgradeAvailable
		pbc.Status.Detail = "v1-21-1004 available"
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateUpgradeAvailable, pbc.Status.State)
		assert.Equal(t, "v1-21-1004 available", pbc.Status.Detail)
	})

	t.Run("upgradeAvailable to upgradeAvailable detail fix", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateUpgradeAvailable
		pbc.Status.Detail = "v1-21-1003 available"
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateUpgradeAvailable, pbc.Status.State)
		assert.Equal(t, "v1-21-1004 available", pbc.Status.Detail)
	})

	t.Run("upgradeAvailable to upgradeAvailable detail fix", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateUpgradeAvailable
		pbc.Status.Detail = "v1-21-1003 available"
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating cluster01 detail to v1-21-1004 available: oops")
	})

	t.Run("upgradeAvailable to active", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateUpgradeAvailable
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.NoError(t, err)
		assert.Equal(t, api.BundleControllerStateActive, pbc.Status.State)
		assert.Equal(t, "", pbc.Status.Detail)
	})

	t.Run("upgradeAvailable to active error", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateUpgradeAvailable
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating cluster01 status to active: oops")
	})

	t.Run("disconnected to active", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateDisconnected
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
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
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating cluster01 status to active: oops")
	})

	t.Run("nothing to active bundle set", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = ""
		pbc.Spec.ActiveBundle = ""
		latestBundle := givenBundle()
		latestBundle.Name = testNextBundleName
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
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
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().Save(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating cluster01 activeBundle to v1-21-1004: oops")
	})

	t.Run("nothing to active state", func(t *testing.T) {
		tcc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		pbc.Status.State = ""
		latestBundle := givenBundle()
		tcc.EXPECT().GetServerVersion(ctx, pbc.Name).Return(&info, nil)
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
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
		tcc.EXPECT().Initialize(ctx, gomock.Any()).Return(nil)
		tcc.EXPECT().ToRESTConfig().Return(&rest.Config{}, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-packages-bundles", testKubeMajor, testKubeMinor, pbc.Name).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx).Return(allBundles, nil)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(fmt.Errorf("oops"))

		err := bm.ProcessBundleController(ctx, pbc)

		assert.EqualError(t, err, "updating cluster01 status to active: oops")
	})
}

func TestBundleManager_isCompatible(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		minVersion   string
		curVersion   string
		isCompatible bool
	}{
		"same version": {minVersion: "v2.2.2", curVersion: "v2.2.2", isCompatible: true},
		"development is compatible with anything":                {minVersion: "v2.2.2", curVersion: "development", isCompatible: true},
		"newer patch version requirement makes it incompatible":  {minVersion: "v2.2.3", curVersion: "v2.2.2", isCompatible: false},
		"newer minor version requirement makes it incompatible":  {minVersion: "v2.3.2", curVersion: "v2.2.2", isCompatible: false},
		"newer major version requirement makes it incompatible":  {minVersion: "v3.2.2", curVersion: "v2.2.2", isCompatible: false},
		"build info is irrelevant":                               {minVersion: "v2.2.2", curVersion: "v2.2.2+shasum", isCompatible: true},
		"build info is irrelevant also if incompatible":          {minVersion: "v2.2.3", curVersion: "v2.2.2+shasum", isCompatible: false},
		"invalid pkg version is incompatible":                    {minVersion: "v2.2.3", curVersion: "2.2.3", isCompatible: false},
		"both invalid version is compatible, I suppose?":         {minVersion: "2.2.3", curVersion: "2.2.2", isCompatible: true},
		"any version is compatible when no requirements imposed": {minVersion: "", curVersion: "2.2.2", isCompatible: true},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, _, _, bm := givenBundleManager(t)
			latestBundle := givenBundle()
			latestBundle.Spec.MinVersion = test.minVersion
			bm.config.BuildInfo.Version = test.curVersion
			assert.Equal(t, test.isCompatible, bm.isCompatibleWith(latestBundle))
		})
	}
}
