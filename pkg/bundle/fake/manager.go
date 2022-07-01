package fake

import (
	"context"

	"k8s.io/apimachinery/pkg/types"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/bundle"
)

type FakeBundleManager struct {
	bundle.Manager
	FakeActiveBundleError             error
	FakeActiveBundle                  *api.PackageBundle
	FakeDownloadBundle                *api.PackageBundle
	FakeUpdate                        bool
	FakeGetActiveBundleNamespacedName types.NamespacedName
}

var _ bundle.Manager = (*FakeBundleManager)(nil)

func NewBundleManager() *FakeBundleManager {
	return &FakeBundleManager{}
}

func (bm *FakeBundleManager) DownloadBundle(ctx context.Context, ref string) (
	*api.PackageBundle, error) {

	if bm.FakeActiveBundleError != nil {
		return nil, bm.FakeActiveBundleError
	}
	return bm.FakeDownloadBundle, nil
}

func (bm *FakeBundleManager) ProcessBundle(ctx context.Context, bundle *api.PackageBundle) (bool, error) {
	return bm.FakeUpdate, nil
}
