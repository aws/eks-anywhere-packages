package controllers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/controllers/mocks"
	bundleMocks "github.com/aws/eks-anywhere-packages/pkg/bundle/mocks"
	"github.com/aws/eks-anywhere-packages/pkg/file"
)

func TestPackageBundleReconciler_mapBundleReconcileRequests(t *testing.T) {
	ctx := context.Background()
	bundleOne, err := file.GivenPackageBundle("../api/testdata/bundle_one.yaml")
	assert.NoError(t, err)
	bundleTwo, err := file.GivenPackageBundle("../api/testdata/bundle_two.yaml")
	assert.NoError(t, err)
	mockClient := mocks.NewMockClient(gomock.NewController(t))
	mockBundleClient := bundleMocks.NewMockClient(gomock.NewController(t))
	mockClient.EXPECT().
		List(ctx, gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, bundles *api.PackageBundleList,
			_ ...*client.ListOptions) error {
			bundles.Items = []api.PackageBundle{*bundleOne, *bundleTwo}
			return nil
		})
	bm := bundleMocks.NewMockManager(gomock.NewController(t))
	sut := NewPackageBundleReconciler(mockClient, nil, mockBundleClient, bm, nil, logr.Discard())

	requests := sut.mapBundleReconcileRequests(&api.PackageBundleController{})

	assert.Equal(t, 2, len(requests))

}
