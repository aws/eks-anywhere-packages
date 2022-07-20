package controllers

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/controllers/mocks"
	"github.com/aws/eks-anywhere-packages/pkg/bundle"
	bundleMocks "github.com/aws/eks-anywhere-packages/pkg/bundle/mocks"
	"github.com/aws/eks-anywhere-packages/pkg/testutil"
)

const testBundleName = "v1.21-1001"

func givenPackageBundleController() api.PackageBundleController {
	return api.PackageBundleController{
		ObjectMeta: metav1.ObjectMeta{
			Name:      api.PackageBundleControllerName,
			Namespace: api.PackageNamespace,
		},
		Spec: api.PackageBundleControllerSpec{
			ActiveBundle: testBundleName,
			Source: api.PackageBundleControllerSource{
				Registry: "public.ecr.aws/j0a1m4z9",
			},
		},
		Status: api.PackageBundleControllerStatus{
			State: api.BundleControllerStateActive,
		},
	}
}

func TestPackageBundleControllerReconcilerReconcile(t *testing.T) {
	t.Parallel()

	discovery := testutil.NewFakeDiscoveryWithDefaults()

	puller := testutil.NewMockPuller()
	mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
	bm := bundle.NewBundleManager(logr.Discard(), discovery, puller, mockBundleClient)

	controllerNN := types.NamespacedName{
		Namespace: api.PackageNamespace,
		Name:      api.PackageBundleControllerName,
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
		if err != nil {
			t.Errorf("expected no error, got %s", err)
		}
		assert.True(t, result.Requeue)
	})

	t.Run("process error", func(t *testing.T) {
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
		if err != nil {
			t.Errorf("expected no error, got %s", err)
		}
		assert.True(t, result.Requeue)
	})

	t.Run("marks status ignored bogus name", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		pbc := givenPackageBundleController()
		pbc.Name = "bogus"
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
				if pbc.Status.State != api.BundleControllerStateIgnored {
					t.Errorf("expected state to be set to Ignored, got %q",
						pbc.Status.State)
				}
				return nil
			})

		r := NewPackageBundleControllerReconciler(mockClient, nil, bm,
			logr.Discard())
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: ignoredController})
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		assert.False(t, result.Requeue)
	})

	t.Run("status error ignored bogus name", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		pbc := givenPackageBundleController()
		pbc.Name = "bogus"
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
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		assert.False(t, result.Requeue)
	})

	t.Run("already ignored bogus name", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		pbc := givenPackageBundleController()
		pbc.Status.State = api.BundleControllerStateIgnored
		pbc.Name = "bogus"
		ignoredController := types.NamespacedName{
			Namespace: api.PackageNamespace,
			Name:      "bogus",
		}
		mockClient.EXPECT().Get(ctx, ignoredController, gomock.Any()).
			DoAndReturn(setMockPBC(&pbc))

		r := NewPackageBundleControllerReconciler(mockClient, nil, bm,
			logr.Discard())
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: ignoredController})
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
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
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
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
		if err == nil {
			t.Fatalf("expected error, got %s", err)
		}
		assert.True(t, result.Requeue)
	})
}
