package controllers

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"gotest.tools/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/modelrocket-add-ons/api/v1alpha1"
	"github.com/aws/modelrocket-add-ons/controllers/mocks"
	bundlefake "github.com/aws/modelrocket-add-ons/pkg/bundle/fake"
)

func TestPackageBundleReconciler_mapBundleReconcileReqeusts(t *testing.T) {
	ctx := context.Background()
	bundleOne := *api.MustPackageBundleFromFilename(t, "../api/testdata/bundle_one.yaml")
	bundleTwo := *api.MustPackageBundleFromFilename(t, "../api/testdata/bundle_two.yaml")
	mockClient := mocks.NewMockClient(gomock.NewController(t))
	mockClient.EXPECT().
		List(ctx, gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, bundles *api.PackageBundleList,
			_ ...client.ListOptions) error {
			bundles.Items = []api.PackageBundle{bundleOne, bundleTwo}
			return nil
		})
	bm := bundlefake.NewBundleManager()
	sut := NewPackageBundleReconciler(mockClient, nil, bm, nil)

	requests := sut.mapBundleReconcileRequests(&api.PackageBundleController{})

	assert.Equal(t, 2, len(requests))

}
