package controllers

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/controllers/mocks"
	mocks2 "github.com/aws/eks-anywhere-packages/pkg/authenticator/mocks"
	"github.com/aws/eks-anywhere-packages/pkg/bundle"
	bundleMocks "github.com/aws/eks-anywhere-packages/pkg/bundle/mocks"
)

const testBundleName = "v1.21-1001"

func givenPackageBundleController() api.PackageBundleController {
	return api.PackageBundleController{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eksa-packages-cluster01",
			Namespace: api.PackageNamespace,
		},
		Spec: api.PackageBundleControllerSpec{
			ActiveBundle: testBundleName,
		},
		Status: api.PackageBundleControllerStatus{
			State: api.BundleControllerStateActive,
		},
	}
}

func TestPackageBundleControllerReconcilerReconcile(t *testing.T) {
	t.Parallel()

	tcc := mocks2.NewMockTargetClusterClient(gomock.NewController(t))
	rc := bundleMocks.NewMockRegistryClient(gomock.NewController(t))
	bc := bundleMocks.NewMockClient(gomock.NewController(t))
	bm := bundle.NewBundleManager(logr.Discard(), rc, bc, tcc)

	controllerNN := types.NamespacedName{
		Namespace: api.PackageNamespace,
		Name:      "eksa-packages-cluster01",
	}
	req := ctrl.Request{
		NamespacedName: controllerNN,
	}

	setMockPBC := func(src *api.PackageBundleController) func(ctx context.Context,
		name types.NamespacedName, pbc *api.PackageBundleController) error {
		return func(ctx context.Context, name types.NamespacedName,
			target *api.PackageBundleController) error {
			src.DeepCopyInto(target)
			return nil
		}
	}

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		pbc := givenPackageBundleController()

		mockClient.EXPECT().Get(ctx, req.NamespacedName, gomock.Any()).
			DoAndReturn(setMockPBC(&pbc))
		mockBundleManager := bundleMocks.NewMockManager(gomock.NewController(t))
		mockBundleManager.EXPECT().ProcessBundleController(ctx, &pbc).Return(nil)

		r := NewPackageBundleControllerReconciler(mockClient, nil, mockBundleManager,
			logr.Discard())
		result, err := r.Reconcile(ctx, req)
		assert.NoError(t, err)
		assert.True(t, result.Requeue)
	})

	t.Run("bundle manager process bundle controller error", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		pbc := givenPackageBundleController()

		mockClient.EXPECT().Get(ctx, req.NamespacedName, gomock.Any()).
			DoAndReturn(setMockPBC(&pbc))
		mockBundleManager := bundleMocks.NewMockManager(gomock.NewController(t))
		mockBundleManager.EXPECT().ProcessBundleController(ctx, &pbc).Return(fmt.Errorf("oops"))

		r := NewPackageBundleControllerReconciler(mockClient, nil, mockBundleManager,
			logr.Discard())
		result, err := r.Reconcile(ctx, req)
		assert.NoError(t, err)
		assert.True(t, result.Requeue)
	})

	t.Run("marks status ignored for bogus package bundle controller namespace", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		pbc := givenPackageBundleController()
		pbc.Namespace = "bogus"
		ignoredController := types.NamespacedName{
			Namespace: api.PackageNamespace,
			Name:      "bogus",
		}
		mockClient.EXPECT().Get(ctx, ignoredController, gomock.Any()).
			DoAndReturn(setMockPBC(&pbc))
		mockStatusClient := mocks.NewMockStatusWriter(gomock.NewController(t))
		mockClient.EXPECT().Status().Return(mockStatusClient)
		mockStatusClient.EXPECT().Update(ctx, gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, pbc *api.PackageBundleController,
				opts *client.UpdateOptions) error {
				assert.Equal(t, pbc.Status.State, api.BundleControllerStateIgnored)
				return nil
			})

		r := NewPackageBundleControllerReconciler(mockClient, nil, bm,
			logr.Discard())
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: ignoredController})
		assert.NoError(t, err)
		assert.False(t, result.Requeue)
	})

	t.Run("error marking status ignored for bogus package bundle controller namespace", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		pbc := givenPackageBundleController()
		pbc.Namespace = "bogus"
		ignoredController := types.NamespacedName{
			Namespace: api.PackageNamespace,
			Name:      "bogus",
		}
		mockClient.EXPECT().Get(ctx, ignoredController, gomock.Any()).
			DoAndReturn(setMockPBC(&pbc))
		mockStatusClient := mocks.NewMockStatusWriter(gomock.NewController(t))
		mockClient.EXPECT().Status().Return(mockStatusClient)
		mockStatusClient.EXPECT().Update(ctx, gomock.Any(), gomock.Any()).Return(fmt.Errorf("oops"))

		r := NewPackageBundleControllerReconciler(mockClient, nil, bm,
			logr.Discard())
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: ignoredController})
		assert.NoError(t, err)
		assert.False(t, result.Requeue)
	})

	t.Run("ignore already ignored bogus package bundle controller", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateIgnored
		pbc.Namespace = "bogus"
		ignoredController := types.NamespacedName{
			Namespace: api.PackageNamespace,
			Name:      "bogus",
		}
		mockClient.EXPECT().Get(ctx, ignoredController, gomock.Any()).
			DoAndReturn(setMockPBC(&pbc))

		r := NewPackageBundleControllerReconciler(mockClient, nil, bm,
			logr.Discard())
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: ignoredController})
		assert.NoError(t, err)
		assert.False(t, result.Requeue)
	})

	t.Run("handles deleted package bundle controllers", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		mockClient := mocks.NewMockClient(gomock.NewController(t))

		groupResource := schema.GroupResource{
			Group:    req.Namespace,
			Resource: req.Name,
		}
		notFoundError := errors.NewNotFound(groupResource, req.Name)
		mockClient.EXPECT().Get(ctx, req.NamespacedName, gomock.Any()).
			Return(notFoundError)

		r := NewPackageBundleControllerReconciler(mockClient, nil, bm,
			logr.Discard())
		result, err := r.Reconcile(ctx, req)
		assert.NoError(t, err)
		assert.False(t, result.Requeue)
	})

	t.Run("get error", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		mockClient.EXPECT().Get(ctx, req.NamespacedName, gomock.Any()).
			Return(fmt.Errorf("oops"))

		r := NewPackageBundleControllerReconciler(mockClient, nil, bm,
			logr.Discard())
		result, err := r.Reconcile(ctx, req)
		assert.EqualError(t, err, "retrieving package bundle controller: oops")
		assert.True(t, result.Requeue)
	})
}
