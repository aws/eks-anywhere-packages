package controllers

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	ctrlmocks "github.com/aws/eks-anywhere-packages/controllers/mocks"
	bundlefake "github.com/aws/eks-anywhere-packages/pkg/bundle/fake"
	bundleClientMocks "github.com/aws/eks-anywhere-packages/pkg/bundle/mocks"
	drivermocks "github.com/aws/eks-anywhere-packages/pkg/driver/mocks"
	"github.com/aws/eks-anywhere-packages/pkg/packages"
	packageMocks "github.com/aws/eks-anywhere-packages/pkg/packages/mocks"
)

func TestReconcile(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		tf, ctx := newTestFixtures(t)

		tf.bundleClient.EXPECT().GetActiveBundle(gomock.Any()).Return(tf.mockBundle(), nil)

		fn, pkg := tf.mockGetFnPkg()
		tf.ctrlClient.EXPECT().
			Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pkg)).
			DoAndReturn(fn)

		tf.bundleManager.FakeActiveBundle = tf.mockBundle()
		tf.packageManager.EXPECT().
			Process(gomock.Any()).
			Return(true)

		status := tf.mockStatusWriter()
		pkg.Status.TargetVersion = "0.1.1"
		status.EXPECT().
			Update(ctx, pkg).
			Return(nil)

		tf.ctrlClient.EXPECT().
			Status().
			Return(status)

		sut := tf.newReconciler()
		req := tf.mockRequest()
		got, err := sut.Reconcile(ctx, req)
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}

		expected := time.Duration(0)
		if got.RequeueAfter != expected {
			t.Errorf("expected <%s> got <%s>", expected, got.RequeueAfter)
		}
	})

	t.Run("happy path no status update", func(t *testing.T) {
		tf, ctx := newTestFixtures(t)

		tf.bundleClient.EXPECT().GetActiveBundle(gomock.Any()).Return(tf.mockBundle(), nil)

		fn, pkg := tf.mockGetFnPkg()
		tf.ctrlClient.EXPECT().
			Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pkg)).
			DoAndReturn(fn)

		tf.bundleManager.FakeActiveBundle = tf.mockBundle()

		tf.packageManager.EXPECT().
			Process(gomock.Any()).
			Return(false)

		sut := tf.newReconciler()
		req := tf.mockRequest()
		got, err := sut.Reconcile(ctx, req)
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}

		expected := time.Duration(0)
		if got.RequeueAfter != expected {
			t.Errorf("expected <%s> got <%s>", expected, got.RequeueAfter)
		}
	})

	t.Run("handles errors getting the package", func(t *testing.T) {
		tf, ctx := newTestFixtures(t)

		testErr := errors.New("getting package test error")
		pkg := tf.mockPackage()
		tf.ctrlClient.EXPECT().
			Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pkg)).
			Return(testErr)

		sut := tf.newReconciler()
		req := tf.mockRequest()
		got, err := sut.Reconcile(ctx, req)
		if err == nil || err.Error() != "getting package test error" {
			t.Fatalf("expected test error, got nil")
		}

		expected := time.Duration(0)
		if got.RequeueAfter != expected {
			t.Errorf("expected <%s> got <%s>", expected, got.RequeueAfter)
		}
	})

	t.Run("handles errors getting the active bundle", func(t *testing.T) {
		tf, ctx := newTestFixtures(t)

		testErr := errors.New("active bundle test error")
		tf.bundleClient.EXPECT().GetActiveBundle(gomock.Any()).Return(nil, testErr)

		fn, pkg := tf.mockGetFnPkg()
		tf.ctrlClient.EXPECT().
			Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pkg)).
			DoAndReturn(fn)

		sut := tf.newReconciler()
		req := tf.mockRequest()
		got, err := sut.Reconcile(ctx, req)
		if err == nil || err.Error() != "active bundle test error" {
			t.Fatalf("expected test error, got nil")
		}

		expected := retryLong
		if got.RequeueAfter != expected {
			t.Errorf("expected <%s> got <%s>", expected, got.RequeueAfter)
		}
	})

	t.Run("handles errors updating status", func(t *testing.T) {
		tf, ctx := newTestFixtures(t)

		fn, pkg := tf.mockGetFnPkg()
		tf.ctrlClient.EXPECT().
			Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pkg)).
			DoAndReturn(fn)
		tf.bundleClient.EXPECT().GetActiveBundle(gomock.Any()).Return(tf.mockBundle(), nil)

		tf.packageManager.EXPECT().
			Process(gomock.Any()).
			Return(true)

		testErr := errors.New("status update test error")
		status := tf.mockStatusWriter()
		pkg.Status.TargetVersion = "0.1.1"
		status.EXPECT().
			Update(ctx, pkg).
			Return(testErr)

		tf.ctrlClient.EXPECT().
			Status().
			Return(status)

		sut := tf.newReconciler()
		req := tf.mockRequest()
		got, err := sut.Reconcile(ctx, req)
		if err == nil || err.Error() != "status update test error" {
			t.Fatalf("expected status update test error, got nil")
		}

		expected := time.Duration(0)
		if got.RequeueAfter != expected {
			t.Errorf("expected <%s> got <%s>", expected, got.RequeueAfter)
		}
	})

	t.Run("Reports error when requested package version is not in the bundle", func(t *testing.T) {
		tf, ctx := newTestFixtures(t)

		fn, pkg := tf.mockGetFnPkg()
		tf.ctrlClient.EXPECT().
			Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pkg)).
			DoAndReturn(fn)

		newBundle := tf.mockBundle()
		newBundle.ObjectMeta.Name = "fake bundle"
		tf.bundleClient.EXPECT().GetActiveBundle(gomock.Any()).Return(newBundle, nil)

		testErr := errors.New("status update test error")
		status := tf.mockStatusWriter()

		pkg.Spec.PackageVersion = "2.0.0"
		pkg.Status.TargetVersion = "2.0.0"
		pkg.Status.Detail = fmt.Sprintf("Package %s@%s is not in the active bundle (%s). Did you forget to activate the new bundle?", pkg.Spec.PackageName, pkg.Spec.PackageVersion, "fake bundle")
		status.EXPECT().
			Update(ctx, pkg).
			Return(testErr)

		tf.ctrlClient.EXPECT().
			Status().
			Return(status)

		sut := tf.newReconciler()
		req := tf.mockRequest()
		got, err := sut.Reconcile(ctx, req)

		assert.Error(t, err, testErr)
		expected := time.Duration(0)
		assert.Equal(t, expected, got.RequeueAfter)

	})

	t.Run("Packages without version hold upgrade to latest", func(t *testing.T) {
		tf, ctx := newTestFixtures(t)

		fn, pkg := tf.mockGetFnPkg()
		tf.ctrlClient.EXPECT().
			Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pkg)).
			DoAndReturn(fn).Times(2)
		pkg.Spec.PackageVersion = ""

		tf.bundleClient.EXPECT().GetActiveBundle(gomock.Any()).Return(tf.mockBundle(), nil)

		newBundle := tf.mockBundle()
		newBundle.Spec.Packages[0].Source.Versions = []api.SourceVersion{{
			Name:   "0.2.0",
			Digest: "sha256:deadbeef020",
		}}
		tf.bundleClient.EXPECT().GetActiveBundle(gomock.Any()).Return(newBundle, nil)

		tf.packageManager.EXPECT().
			Process(gomock.Any()).
			Return(false).Do(func(mctx *packages.ManagerContext) {
			assert.Equal(t,
				api.PackageOCISource(api.PackageOCISource{Version: "0.1.1", Registry: "public.ecr.aws/l0g8r8j6", Repository: "hello-eks-anywhere", Digest: "sha256:deadbeef"}),
				mctx.Source)
		})

		sut := tf.newReconciler()
		req := tf.mockRequest()
		got, err := sut.Reconcile(ctx, req)
		assert.NoError(t, err)
		expected := time.Duration(0)
		assert.Equal(t, expected, got.RequeueAfter)

		tf.bundleManager.FakeActiveBundle = newBundle
		tf.packageManager.EXPECT().
			Process(gomock.Any()).
			Return(false).Do(func(mctx *packages.ManagerContext) {
			assert.Equal(t,
				api.PackageOCISource(api.PackageOCISource{Version: "0.2.0", Registry: "public.ecr.aws/l0g8r8j6", Repository: "hello-eks-anywhere", Digest: "sha256:deadbeef020"}),
				mctx.Source)
		})

		got, err = sut.Reconcile(ctx, req)
		assert.NoError(t, err)
		expected = time.Duration(0)
		assert.Equal(t, expected, got.RequeueAfter)

	})
}

//
// Test helpers
//

const (
	name      string = "Yoda"
	namespace string = "Jedi"
)

type testFixtures struct {
	gomockController *gomock.Controller
	logger           logr.Logger

	ctrlClient     *ctrlmocks.MockClient
	packageDriver  *drivermocks.MockPackageDriver
	packageManager *packageMocks.MockManager
	bundleManager  *bundlefake.FakeBundleManager
	bundleClient   *bundleClientMocks.MockBundleClient
}

// newTestFixtures helps remove repetition in the tests by instantiating a lot of
// commonly used structures and mocks.
func newTestFixtures(t *testing.T) (*testFixtures, context.Context) {
	gomockController := gomock.NewController(t)
	return &testFixtures{
		gomockController: gomockController,
		logger:           logr.Discard(),
		ctrlClient:       ctrlmocks.NewMockClient(gomockController),
		packageDriver:    drivermocks.NewMockPackageDriver(gomockController),
		packageManager:   packageMocks.NewMockManager(gomockController),
		bundleManager:    bundlefake.NewBundleManager(),
		bundleClient:     bundleClientMocks.NewMockBundleClient(gomockController),
	}, context.Background()
}

func (tf *testFixtures) mockSpec() api.PackageSpec {
	return api.PackageSpec{
		PackageName:    "hello-eks-anywhere",
		PackageVersion: "0.1.1",
		Config: `
config:
  foo: foo
secret:
  bar: bar
`,
	}
}

func (tf *testFixtures) mockPackage() *api.Package {
	return &api.Package{
		TypeMeta: metav1.TypeMeta{Kind: "Package"},
		Spec:     tf.mockSpec(),
	}
}

func (tf *testFixtures) mockBundle() *api.PackageBundle {
	return &api.PackageBundle{
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

func (tf *testFixtures) mockRequest() ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func (tf *testFixtures) newReconciler() *PackageReconciler {
	// copy these default values
	mockCtrlClient := tf.ctrlClient
	mockPackageDriver := tf.packageDriver
	mockPackageManager := tf.packageManager
	mockBundleManager := tf.bundleManager
	mockBundleClient := tf.bundleClient

	return &PackageReconciler{
		Client:        mockCtrlClient,
		Scheme:        nil,
		Log:           tf.logger,
		PackageDriver: mockPackageDriver,
		Manager:       mockPackageManager,
		bundleManager: mockBundleManager,
		bundleClient:  mockBundleClient,
	}
}

type getFnPkg func(context.Context, types.NamespacedName, *api.Package) error

func (tf *testFixtures) mockGetFnPkg() (getFnPkg, *api.Package) {
	pkg := tf.mockPackage()
	return func(ctx context.Context, name types.NamespacedName,
		target *api.Package) error {
		pkg.DeepCopyInto(target)
		return nil
	}, pkg
}

func (tf *testFixtures) mockStatusWriter() *ctrlmocks.MockStatusWriter {
	return ctrlmocks.NewMockStatusWriter(tf.gomockController)
}
