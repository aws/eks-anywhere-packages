package controllers_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/controllers"
	"github.com/aws/eks-anywhere-packages/controllers/mocks"
	bundlefake "github.com/aws/eks-anywhere-packages/pkg/bundle/fake"
	bundleMocks "github.com/aws/eks-anywhere-packages/pkg/bundle/mocks"
)

func givenRequest() ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "some-bundle",
			Namespace: v1alpha1.PackageNamespace,
		},
	}
}

func TestPackageBundleReconciler_ReconcileAddUpdate(t *testing.T) {
	ctx := context.Background()
	request := givenRequest()
	statusWriter := mocks.NewMockStatusWriter(gomock.NewController(t))
	mockClient := mocks.NewMockClient(gomock.NewController(t))
	mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
	mockClient.EXPECT().Get(ctx, request.NamespacedName, gomock.Any()).Return(nil)
	mockClient.EXPECT().Status().Return(statusWriter)
	statusWriter.EXPECT().Update(ctx, gomock.Any(), gomock.Any()).Return(nil)
	bm := bundlefake.NewBundleManager()
	bm.FakeUpdate = true
	sut := controllers.NewPackageBundleReconciler(mockClient, nil, mockBundleClient, bm, logr.Discard())

	_, actualError := sut.Reconcile(ctx, request)

	if actualError != nil {
		t.Errorf("expected <%v> actual <%v>", nil, actualError)
	}
}

func TestPackageBundleReconciler_ReconcileError(t *testing.T) {
	ctx := context.Background()
	request := givenRequest()
	mockClient := mocks.NewMockClient(gomock.NewController(t))
	mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
	expectedError := fmt.Errorf("error reading")
	mockClient.EXPECT().Get(ctx, request.NamespacedName, gomock.Any()).Return(expectedError)
	bm := bundlefake.NewBundleManager()
	bm.FakeUpdate = false
	sut := controllers.NewPackageBundleReconciler(mockClient, nil, mockBundleClient, bm, logr.Discard())

	_, actualError := sut.Reconcile(ctx, request)

	if actualError == nil || actualError != expectedError {
		t.Errorf("expected <%v> actual <%v>", expectedError, actualError)
	}
}

func TestPackageBundleReconciler_ReconcileIgnored(t *testing.T) {
	ctx := context.Background()
	request := givenRequest()
	request.Name = "bogus"
	mockClient := mocks.NewMockClient(gomock.NewController(t))
	mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
	mockClient.EXPECT().Get(ctx, request.NamespacedName, gomock.Any()).Return(nil)
	bm := bundlefake.NewBundleManager()
	bm.FakeUpdate = false
	sut := controllers.NewPackageBundleReconciler(mockClient, nil, mockBundleClient, bm, logr.Discard())

	_, actualError := sut.Reconcile(ctx, request)

	if actualError != nil {
		t.Errorf("expected <%v> actual <%v>", nil, actualError)
	}
}

func TestPackageBundleReconciler_ReconcileDelete(t *testing.T) {
	ctx := context.Background()
	request := givenRequest()
	groupResource := schema.GroupResource{
		Group:    request.Namespace,
		Resource: request.Name,
	}
	notFoundError := errors.NewNotFound(groupResource, request.Name)
	mockClient := mocks.NewMockClient(gomock.NewController(t))
	mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
	mockClient.EXPECT().Get(ctx, request.NamespacedName, gomock.Any()).Return(notFoundError)
	mockBundleClient.EXPECT().GetActiveBundleNamespacedName(ctx).Return(request.NamespacedName, nil)
	bm := bundlefake.NewBundleManager()
	mockClient.EXPECT().Create(ctx, bm.FakeActiveBundle).Return(nil)
	sut := controllers.NewPackageBundleReconciler(mockClient, nil, mockBundleClient, bm, logr.Discard())

	_, actualError := sut.Reconcile(ctx, request)

	if actualError != nil {
		t.Errorf("expected <%v> actual <%v>", nil, actualError)
	}
}
