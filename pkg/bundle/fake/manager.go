package fake

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/bundle"
)

type FakeBundleManager struct {
	bundle.Manager
	FakeActiveBundleError             error
	FakeActiveBundle                  *api.PackageBundle
	FakeDownloadBundle                *api.PackageBundle
	FakeIsActive                      bool
	FakeUpdate                        bool
	FakeGetActiveBundleNamespacedName types.NamespacedName
}

var _ bundle.Manager = (*FakeBundleManager)(nil)

func NewBundleManager() *FakeBundleManager {
	return &FakeBundleManager{}
}

func (bm *FakeBundleManager) ActiveBundle(ctx context.Context,
	client client.Client) (*api.PackageBundle, error) {

	if bm.FakeActiveBundleError != nil {
		return nil, bm.FakeActiveBundleError
	}
	return bm.FakeActiveBundle, nil
}

func (bm *FakeBundleManager) DownloadBundle(ctx context.Context, ref string) (
	*api.PackageBundle, error) {

	if bm.FakeActiveBundleError != nil {
		return nil, bm.FakeActiveBundleError
	}
	return bm.FakeDownloadBundle, nil
}

func (bm *FakeBundleManager) IsActive(ctx context.Context,
	client client.Client, name types.NamespacedName) (bool, error) {
	return bm.FakeIsActive, nil
}

func (bm *FakeBundleManager) Update(bundle *api.PackageBundle, active bool,
	allBundles []api.PackageBundle) bool {
	return bm.FakeUpdate
}
