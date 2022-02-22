package controllers_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/aws/eks-anywhere-packages/controllers"
	"github.com/aws/eks-anywhere-packages/controllers/mocks"
	"github.com/aws/eks-anywhere-packages/pkg/bundle"
	bundlefake "github.com/aws/eks-anywhere-packages/pkg/bundle/fake"
)

func givenRequest() ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "some-bundle",
			Namespace: bundle.ActiveBundleNamespace,
		},
	}
}

func TestPackageBundleReconciler_ReconcileAddUpdate(t *testing.T) {
	ctx := context.Background()
	request := givenRequest()
	statusWriter := mocks.NewMockStatusWriter(gomock.NewController(t))
	mockClient := mocks.NewMockClient(gomock.NewController(t))
	mockClient.EXPECT().Get(ctx, request.NamespacedName, gomock.Any()).Return(nil)
	mockClient.EXPECT().Status().Return(statusWriter)
	mockClient.EXPECT().List(ctx, gomock.Any()).Return(nil)
	statusWriter.EXPECT().Update(ctx, gomock.Any(), gomock.Any()).Return(nil)
	bm := bundlefake.NewBundleManager()
	bm.FakeIsActive = true
	bm.FakeUpdate = true
	sut := controllers.NewPackageBundleReconciler(mockClient, nil, bm, nil)

	_, actualError := sut.Reconcile(ctx, request)

	if actualError != nil {
		t.Errorf("expected <%v> actual <%v>", nil, actualError)
	}
}

func TestPackageBundleReconciler_ReconcileError(t *testing.T) {
	ctx := context.Background()
	request := givenRequest()
	mockClient := mocks.NewMockClient(gomock.NewController(t))
	expectedError := fmt.Errorf("error reading")
	mockClient.EXPECT().Get(ctx, request.NamespacedName, gomock.Any()).Return(expectedError)
	bm := bundlefake.NewBundleManager()
	bm.FakeIsActive = true
	sut := controllers.NewPackageBundleReconciler(mockClient, nil, bm, nil)

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
	mockClient.EXPECT().Get(ctx, request.NamespacedName, gomock.Any()).Return(nil)
	mockClient.EXPECT().List(ctx, gomock.Any()).Return(nil)
	bm := bundlefake.NewBundleManager()
	sut := controllers.NewPackageBundleReconciler(mockClient, nil, bm, nil)

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
	mockClient.EXPECT().Get(ctx, request.NamespacedName, gomock.Any()).Return(notFoundError)
	bm := bundlefake.NewBundleManager()
	bm.FakeIsActive = true
	sut := controllers.NewPackageBundleReconciler(mockClient, nil, bm, nil)

	_, actualError := sut.Reconcile(ctx, request)

	if actualError != nil {
		t.Errorf("expected <%v> actual <%v>", nil, actualError)
	}
}
