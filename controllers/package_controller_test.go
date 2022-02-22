package controllers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/aws/modelrocket-add-ons/api/v1alpha1"
	ctrlmocks "github.com/aws/modelrocket-add-ons/controllers/mocks"
	bundlefake "github.com/aws/modelrocket-add-ons/pkg/bundle/fake"
	drivermocks "github.com/aws/modelrocket-add-ons/pkg/driver/mocks"
	packageMocks "github.com/aws/modelrocket-add-ons/pkg/packages/mocks"
)

func TestReconcile(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		tf, ctx := newTestFixtures(t)

		fn, pkg := tf.mockGetFnPkg()
		tf.ctrlClient.EXPECT().
			Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pkg)).
			DoAndReturn(fn)

		tf.bundleManager.FakeActiveBundle = tf.mockBundle()

		tf.packageManager.EXPECT().
			Process(gomock.Any()).
			Return(true)

		status := tf.mockStatusWriter()
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

		fn, pkg := tf.mockGetFnPkg()
		tf.ctrlClient.EXPECT().
			Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(pkg)).
			DoAndReturn(fn)

		testErr := errors.New("active bundle test error")
		tf.bundleManager.FakeActiveBundleError = testErr

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

		tf.bundleManager.FakeActiveBundle = tf.mockBundle()

		tf.packageManager.EXPECT().
			Process(gomock.Any()).
			Return(true)

		testErr := errors.New("status update test error")
		status := tf.mockStatusWriter()
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
}

//
// Test helpers
//

const (
	name        string = "Yoda"
	namespace   string = "Jedi"
	configValue string = "foo"
	secretValue string = "bar"
)

type testFixtures struct {
	gomockController *gomock.Controller
	logger           log.NullLogger

	ctrlClient     *ctrlmocks.MockClient
	packageDriver  *drivermocks.MockPackageDriver
	packageManager *packageMocks.MockManager
	bundleManager  *bundlefake.FakeBundleManager
}

// newTestFixtures helps remove repetition in the tests by instantiating a lot of
// commonly used structures and mocks.
func newTestFixtures(t *testing.T) (*testFixtures, context.Context) {
	gomockController := gomock.NewController(t)
	return &testFixtures{
		gomockController: gomockController,
		logger:           log.NullLogger{},
		ctrlClient:       ctrlmocks.NewMockClient(gomockController),
		packageDriver:    drivermocks.NewMockPackageDriver(gomockController),
		packageManager:   packageMocks.NewMockManager(gomockController),
		bundleManager:    bundlefake.NewBundleManager(),
	}, context.Background()
}

func (tf *testFixtures) mockSpec() api.PackageSpec {
	return api.PackageSpec{
		PackageName:    "eks-anywhere-test",
		PackageVersion: "v0.1.1",
		Config: map[string]string{
			"config.foo": configValue,
			"secret.bar": secretValue,
		},
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
					Name: "eks-anywhere-test",
					Source: api.BundlePackageSource{
						Registry:   "public.ecr.aws/l0g8r8j6",
						Repository: "eks-anywhere-test",
						Versions: []api.SourceVersion{
							{Name: "v0.1.1", Tag: "sha256:deadbeef"},
							{Name: "v0.1.0", Tag: "sha256:cafebabe"},
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

	return &PackageReconciler{
		Client:        mockCtrlClient,
		Scheme:        nil,
		Log:           tf.logger,
		PackageDriver: mockPackageDriver,
		Manager:       mockPackageManager,
		bundleManager: mockBundleManager,
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
