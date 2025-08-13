package controllers

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
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
	"github.com/aws/eks-anywhere-packages/pkg/config"
	"github.com/aws/eks-anywhere-packages/pkg/registry"
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

	cfg := config.GetConfig()
	tcc := mocks2.NewMockTargetClusterClient(gomock.NewController(t))
	rc := bundleMocks.NewMockRegistryClient(gomock.NewController(t))
	bc := bundleMocks.NewMockClient(gomock.NewController(t))
	bm := bundle.NewBundleManager(logr.Discard(), rc, bc, tcc, cfg)

	controllerNN := types.NamespacedName{
		Namespace: api.PackageNamespace,
		Name:      "eksa-packages-cluster01",
	}
	req := ctrl.Request{
		NamespacedName: controllerNN,
	}

	setMockPBC := func(src *api.PackageBundleController) func(ctx context.Context,
		name types.NamespacedName, pbc *api.PackageBundleController, _ ...client.GetOption) error {
		return func(ctx context.Context, name types.NamespacedName,
			target *api.PackageBundleController, _ ...client.GetOption,
		) error {
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
		ci := registry.NewCertInjector(mockClient, logr.Discard())

		r := NewPackageBundleControllerReconciler(mockClient, nil, mockBundleManager, ci,
			logr.Discard())
		result, err := r.Reconcile(ctx, req)
		assert.NoError(t, err)
		assert.True(t, result.RequeueAfter > 0)
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
		ci := registry.NewCertInjector(mockClient, logr.Discard())

		r := NewPackageBundleControllerReconciler(mockClient, nil, mockBundleManager, ci,
			logr.Discard())
		result, err := r.Reconcile(ctx, req)
		assert.NoError(t, err)
		assert.True(t, result.RequeueAfter > 0)
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
				opts *client.SubResourceUpdateOptions,
			) error {
				assert.Equal(t, pbc.Status.State, api.BundleControllerStateIgnored)
				return nil
			})
		ci := registry.NewCertInjector(mockClient, logr.Discard())

		r := NewPackageBundleControllerReconciler(mockClient, nil, bm, ci,
			logr.Discard())
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: ignoredController})
		assert.NoError(t, err)
		assert.Equal(t, result.RequeueAfter, time.Duration(0))
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
		ci := registry.NewCertInjector(mockClient, logr.Discard())

		r := NewPackageBundleControllerReconciler(mockClient, nil, bm, ci,
			logr.Discard())
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: ignoredController})
		assert.NoError(t, err)
		assert.Equal(t, result.RequeueAfter, time.Duration(0))
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
		ci := registry.NewCertInjector(mockClient, logr.Discard())

		r := NewPackageBundleControllerReconciler(mockClient, nil, bm, ci,
			logr.Discard())
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: ignoredController})
		assert.NoError(t, err)
		assert.Equal(t, result.RequeueAfter, time.Duration(0))
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
		ci := registry.NewCertInjector(mockClient, logr.Discard())

		r := NewPackageBundleControllerReconciler(mockClient, nil, bm, ci,
			logr.Discard())
		result, err := r.Reconcile(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, result.RequeueAfter, time.Duration(0))
	})

	t.Run("get error", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		mockClient.EXPECT().Get(ctx, req.NamespacedName, gomock.Any()).
			Return(fmt.Errorf("oops"))
		ci := registry.NewCertInjector(mockClient, logr.Discard())
		r := NewPackageBundleControllerReconciler(mockClient, nil, bm, ci,
			logr.Discard())
		result, err := r.Reconcile(ctx, req)
		assert.EqualError(t, err, "retrieving package bundle controller: oops")
		assert.True(t, result.RequeueAfter > 0)
	})

	t.Run("package bundle controller with non default image registry with cert update", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		pbc := givenPackageBundleController()
		pbc.Spec.DefaultRegistry = "1.2.3.4:443/ecr-public/eks-anywhere"
		regSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "registry-mirror-secret",
				Namespace: "eksa-packages-eksa-packages-cluster01",
			},
			Data: map[string][]byte{
				"CACERTCONTENT": bytes.NewBufferString("AAA").Bytes(),
			},
		}
		regCredSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "registry-mirror-cred",
				Namespace: api.PackageNamespace,
			},
			Data: make(map[string][]byte),
		}
		returnRegSec := &corev1.Secret{}
		mockClient.EXPECT().Get(ctx, req.NamespacedName, gomock.Any()).
			DoAndReturn(setMockPBC(&pbc))
		mockClient.EXPECT().Get(ctx, types.NamespacedName{Name: regSecret.Name, Namespace: regSecret.Namespace}, returnRegSec).
			DoAndReturn(func(ctx context.Context, name types.NamespacedName, s *corev1.Secret, _ ...client.GetOption) error {
				regSecret.DeepCopyInto(s)
				return nil
			})
		mockClient.EXPECT().Get(ctx, types.NamespacedName{Name: regCredSecret.Name, Namespace: regCredSecret.Namespace}, returnRegSec).
			DoAndReturn(func(ctx context.Context, name types.NamespacedName, s *corev1.Secret, _ ...client.GetOption) error {
				regCredSecret.DeepCopyInto(s)
				return nil
			})
		mockClient.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).Return(nil)
		mockBundleManager := bundleMocks.NewMockManager(gomock.NewController(t))
		mockBundleManager.EXPECT().ProcessBundleController(ctx, &pbc).Return(nil)
		ci := registry.NewCertInjector(mockClient, logr.Discard())

		r := NewPackageBundleControllerReconciler(mockClient, nil, mockBundleManager, ci,
			logr.Discard())
		result, err := r.Reconcile(ctx, req)
		assert.NoError(t, err)
		assert.True(t, result.RequeueAfter > 0)
	})
}
