package bundle

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func givenBundleManager(t *testing.T) (*bundleMocks.MockKubeVersionClient, *bundleMocks.MockRegistryClient, *bundleMocks.MockClient, *bundleManager) {
	kvc := bundleMocks.NewMockKubeVersionClient(gomock.NewController(t))
	rc := bundleMocks.NewMockRegistryClient(gomock.NewController(t))
	bc := bundleMocks.NewMockClient(gomock.NewController(t))
	bm := NewBundleManager(logr.Discard(), kvc, rc, bc)
	return kvc, rc, bc, bm
}

func TestProcessBundle(t *testing.T) {
	ctx := context.Background()

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
		kvc, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateInactive)
		bundle.Name = "v1-01-1"
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		kvc.EXPECT().ApiVersion().Return(testKubernetesVersion, nil)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateIgnoredVersion, bundle.Status.State)
	})

	t.Run("ignore invalid Kubernetes version", func(t *testing.T) {
		kvc, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateInactive)
		bundle.Name = "v1-21-x"
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		kvc.EXPECT().ApiVersion().Return(testKubernetesVersion, nil)

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
		kvc, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateInactive)
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		kvc.EXPECT().ApiVersion().Return(testKubernetesVersion, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleListNone)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateActive, bundle.Status.State)
	})

	t.Run("marks state inactive", func(t *testing.T) {
		kvc, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateActive)
		bundle.Name = testPreviousBundleName
		kvc.EXPECT().ApiVersion().Return(testKubernetesVersion, nil)
		bc.EXPECT().IsActive(ctx, bundle).Return(false, nil)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateInactive, bundle.Status.State)
	})

	t.Run("leaves state as-is (inactive)", func(t *testing.T) {
		kvc, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateInactive)
		bundle.Name = testPreviousBundleName
		kvc.EXPECT().ApiVersion().Return(testKubernetesVersion, nil)
		bc.EXPECT().IsActive(ctx, bundle).Return(false, nil)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.False(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateInactive, bundle.Status.State)
	})

	t.Run("leaves state as-is (active) empty list", func(t *testing.T) {
		kvc, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateActive)
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		kvc.EXPECT().ApiVersion().Return(testKubernetesVersion, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleListNone)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.False(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateActive, bundle.Status.State)
	})

	t.Run("leaves state as-is (active)", func(t *testing.T) {
		kvc, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateActive)
		bundle.Status.State = api.PackageBundleStateActive
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		kvc.EXPECT().ApiVersion().Return(testKubernetesVersion, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.False(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateActive, bundle.Status.State)
	})

	t.Run("marks state upgrade available", func(t *testing.T) {
		kvc, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateActive)
		bundle.Name = testNextBundleName
		bundle.Status.State = api.PackageBundleStateActive
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		kvc.EXPECT().ApiVersion().Return(testKubernetesVersion, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.True(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateUpgradeAvailable, bundle.Status.State)
	})

	t.Run("leaves state as-is (upgrade available)", func(t *testing.T) {
		kvc, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateUpgradeAvailable)
		bundle.Name = testNextBundleName
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		kvc.EXPECT().ApiVersion().Return(testKubernetesVersion, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)

		update, err := bm.ProcessBundle(ctx, bundle)

		assert.False(t, update)
		assert.Equal(t, nil, err)
		assert.Equal(t, api.PackageBundleStateUpgradeAvailable, bundle.Status.State)
	})

	t.Run("get bundle list error", func(t *testing.T) {
		kvc, _, bc, bm := givenBundleManager(t)
		bundle := GivenBundle(api.PackageBundleStateActive)
		bc.EXPECT().IsActive(ctx, bundle).Return(true, nil)
		kvc.EXPECT().ApiVersion().Return(testKubernetesVersion, nil)
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

	t.Run("upgrade available", func(t *testing.T) {
		kvc, rc, bc, bm := givenBundleManager(t)
		pbc := givenPackageBundleController()
		latestBundle := givenBundle()
		kvc.EXPECT().ApiVersion().Return(testKubernetesVersion, nil)
		rc.EXPECT().LatestBundle(ctx, testBundleRegistry+"/eks-anywhere-package-bundles", testKubernetesVersion).Return(latestBundle, nil)
		bc.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
		bc.EXPECT().CreateBundle(ctx, latestBundle).Return(nil)
		bc.EXPECT().SaveStatus(ctx, pbc).Return(nil)

		err := bm.ProcessBundleController(ctx, pbc)

		assert.Nil(t, err)
		assert.Equal(t, pbc.Status.State, api.BundleControllerStateUpgradeAvailable)
	})

	//t.Run("active bundle correct", func(t *testing.T) {
	//	discovery := testutil.NewFakeDiscoveryWithDefaults()
	//	puller := testutil.NewMockPuller()
	//	pbc := givenPackageBundleController()
	//	mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
	//	bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)
	//	mockBundleClient.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
	//	//mockBundleClient.EXPECT().SaveStatus(ctx, pbc).Return(nil)
	//
	//	err := bm.ProcessBundleController(ctx, pbc)
	//
	//	assert.Nil(t, err)
	//})

	//t.Run("save status error", func(t *testing.T) {
	//	discovery := testutil.NewFakeDiscoveryWithDefaults()
	//	puller := testutil.NewMockPuller()
	//	pbc := givenPackageBundleController()
	//	mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
	//	bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)
	//	mockBundleClient.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
	//	mockBundleClient.EXPECT().SaveStatus(ctx, pbc).Return(fmt.Errorf("oops"))
	//
	//	err := bm.ProcessBundleController(ctx, pbc)
	//
	//	assert.EqualError(t, err, "oops")
	//})
	//
	//t.Run("known bundle", func(t *testing.T) {
	//	discovery := testutil.NewFakeDiscoveryWithDefaults()
	//	puller := testutil.NewMockPuller()
	//	pbc := givenPackageBundleController()
	//	mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
	//	mockBundleClient.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
	//	bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)
	//
	//	err := bm.ProcessBundleController(ctx, pbc)
	//
	//	assert.Nil(t, err)
	//})
	//
	//t.Run("bundle create error", func(t *testing.T) {
	//	discovery := testutil.NewFakeDiscoveryWithDefaults()
	//	puller := testutil.NewMockPuller()
	//	pbc := givenPackageBundleController()
	//	//bundle := GivenBundle(api.PackageBundleStateInactive)
	//	//bundle.Namespace = "eksa-packages"
	//	//bundle.Name = testNextBundleName
	//	mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
	//	bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)
	//	mockBundleClient.EXPECT().GetBundleList(ctx, gomock.Any()).DoAndReturn(mockGetBundleList)
	//	//mockBundleClient.EXPECT().CreateBundle(ctx, bundle).Return(fmt.Errorf("oops"))
	//
	//	err := bm.ProcessBundleController(ctx, pbc)
	//
	//	assert.EqualError(t, err, "creating new package bundle: oops")
	//})
	//
	//t.Run("bundle list error", func(t *testing.T) {
	//	discovery := testutil.NewFakeDiscoveryWithDefaults()
	//	puller := testutil.NewMockPuller()
	//	pbc := givenPackageBundleController()
	//	//bundle := GivenBundle(api.PackageBundleStateInactive)
	//	mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
	//	mockBundleClient.EXPECT().GetBundleList(ctx, gomock.Any()).Return(fmt.Errorf("oops"))
	//	bm := NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)
	//
	//	err := bm.ProcessBundleController(ctx, pbc)
	//
	//	assert.EqualError(t, err, "getting bundle list: oops")
	//})

	/*
		t.Run("marks status disconnected if active", func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			mockClient := mocks.NewMockClient(gomock.NewController(t))
			pbc := givenPackageBundleController()
			pbc.Status.State = api.BundleControllerStateActive

			mockClient.EXPECT().Get(ctx, req.NamespacedName, gomock.Any()).
				DoAndReturn(setMockPBC(&pbc))
			mockBundleManager := bundleMocks.NewMockManager(gomock.NewController(t))
			mockStatusClient := mocks.NewMockStatusWriter(gomock.NewController(t))
			mockClient.EXPECT().Status().Return(mockStatusClient)
			mockStatusClient.EXPECT().Update(ctx, gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, pbc *api.PackageBundleController,
					opts *client.UpdateOptions) error {
					if pbc.Status.State != api.BundleControllerStateDisconnected {
						t.Errorf("expected state to be set to Active, got %q",
							pbc.Status.State)
					}
					return nil
				})

			r := NewPackageBundleControllerReconciler(mockClient, nil, mockBundleManager,
				logr.Discard())
			_, err := r.Reconcile(ctx, req)
			if err != nil {
				t.Errorf("expected no error, got %s", err)
			}
		})

		t.Run("marks status active if state not set", func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			mockClient := mocks.NewMockClient(gomock.NewController(t))
			pbc := givenPackageBundleController()
			pbc.Status.State = ""

			mockClient.EXPECT().Get(ctx, req.NamespacedName, gomock.Any()).
				DoAndReturn(setMockPBC(&pbc))
			testBundle := api.PackageBundle{}
			mockBundleManager := bundleMocks.NewMockManager(gomock.NewController(t))
			mockBundleManager.EXPECT().LatestBundle(ctx, pbc.Spec.Source.BaseRef()).Return(&testBundle, nil)
			mockBundleManager.EXPECT().ProcessBundleController(ctx, &testBundle).Return(nil)
			mockClient.EXPECT().Update(ctx, gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, pbc *api.PackageBundleController,
					opts *client.UpdateOptions) error {
					if pbc.Status.State != api.BundleControllerStateActive {
						t.Errorf("expected state to be set to Active, got %q",
							pbc.Status.State)
					}
					return nil
				})

			r := NewPackageBundleControllerReconciler(mockClient, nil, mockBundleManager,
				logr.Discard())
			_, err := r.Reconcile(ctx, req)
			if err != nil {
				t.Errorf("expected no error, got %s", err)
			}
		})

		t.Run("marks status active if disconnected", func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			mockClient := mocks.NewMockClient(gomock.NewController(t))
			pbc := givenPackageBundleController()
			pbc.Status.State = api.BundleControllerStateDisconnected

			mockClient.EXPECT().Get(ctx, req.NamespacedName, gomock.Any()).
				DoAndReturn(setMockPBC(&pbc))
			testBundle := api.PackageBundle{}
			mockBundleManager := bundleMocks.NewMockManager(gomock.NewController(t))
			mockBundleManager.EXPECT().ProcessBundleController(ctx, &testBundle).Return(nil)
			mockStatusClient := mocks.NewMockStatusWriter(gomock.NewController(t))
			mockClient.EXPECT().Status().Return(mockStatusClient)
			mockStatusClient.EXPECT().Update(ctx, gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, pbc *api.PackageBundleController,
					opts *client.UpdateOptions) error {
					if pbc.Status.State != api.BundleControllerStateActive {
						t.Errorf("expected state to be set to Active, got %q",
							pbc.Status.State)
					}
					return nil
				})

			r := NewPackageBundleControllerReconciler(mockClient, nil, mockBundleManager,
				logr.Discard())
			_, err := r.Reconcile(ctx, req)
			if err != nil {
				t.Errorf("expected no error, got %s", err)
			}
		})
	*/
}
