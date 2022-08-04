package bundle

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

type Client interface {
	// IsActive returns true if the bundle is the active bundle
	IsActive(ctx context.Context, packageBundle *api.PackageBundle) (bool, error)

	// GetActiveBundle retrieves the currently active bundle.
	GetActiveBundle(ctx context.Context) (activeBundle *api.PackageBundle, err error)

	// GetActiveBundleNamespacedName retrieves the namespace and name of the
	// currently active bundle.
	GetActiveBundleNamespacedName(ctx context.Context) (types.NamespacedName, error)

	// GetBundleList get list of bundles worthy of consideration
	GetBundleList(ctx context.Context, bundles *api.PackageBundleList) error

	// CreateBundle add a new bundle custom resource
	CreateBundle(ctx context.Context, bundle *api.PackageBundle) error

	// SaveStatus saves a resource status
	SaveStatus(ctx context.Context, object client.Object) error

	// Save saves a resource
	Save(ctx context.Context, object client.Object) error
}

type bundleClient struct {
	client.Client
}

func NewPackageBundleClient(client client.Client) *bundleClient {
	return &(bundleClient{
		Client: client,
	})
}

var _ Client = (*bundleClient)(nil)

func (bc *bundleClient) getPackageBundleController(ctx context.Context) (*api.PackageBundleController, error) {
	pbc := api.PackageBundleController{}
	key := types.NamespacedName{
		Namespace: api.PackageNamespace,
		Name:      api.PackageBundleControllerName,
	}
	err := bc.Get(ctx, key, &pbc)
	if err != nil {
		return nil, fmt.Errorf("getting PackageBundleController: %v", err)
	}
	return &pbc, nil
}

// IsActive returns true of the bundle is the active bundle
func (bc *bundleClient) IsActive(ctx context.Context, packageBundle *api.PackageBundle) (bool, error) {

	pbc, err := bc.getPackageBundleController(ctx)
	if err != nil {
		return false, err
	}

	return packageBundle.Namespace == api.PackageNamespace && packageBundle.Name == pbc.Spec.ActiveBundle, nil
}

// GetActiveBundle retrieves the bundle from which package are installed.
//
// It retrieves the name of the active bundle from the PackageBundleController,
// then uses the K8s API to retrieve and return the active bundle.
func (bc *bundleClient) GetActiveBundle(ctx context.Context) (activeBundle *api.PackageBundle, err error) {
	pbc, err := bc.getPackageBundleController(ctx)
	if err != nil {
		return nil, err
	}

	if pbc.Spec.ActiveBundle == "" {
		return nil, fmt.Errorf("no activeBundle set in PackageBundleController")
	}

	nn := types.NamespacedName{
		Namespace: api.PackageNamespace,
		Name:      pbc.Spec.ActiveBundle,
	}
	activeBundle = &api.PackageBundle{}
	err = bc.Get(ctx, nn, activeBundle)
	if err != nil {
		return nil, err
	}

	for i, bundlePackage := range activeBundle.Spec.Packages {
		if len(bundlePackage.Source.Registry) < 1 {
			if len(pbc.Spec.Source.Registry) < 1 {
				activeBundle.Spec.Packages[i].Source.Registry = api.DefaultPackageRegistry
			} else {
				activeBundle.Spec.Packages[i].Source.Registry = pbc.Spec.Source.Registry
			}
		}
	}

	return activeBundle, nil
}

// GetActiveBundleNamespacedName retrieves the namespace and name of the
// currently active bundle from the PackageBundleController.
func (bc *bundleClient) GetActiveBundleNamespacedName(ctx context.Context) (types.NamespacedName, error) {
	pbc, err := bc.getPackageBundleController(ctx)
	if err != nil {
		return types.NamespacedName{}, err
	}

	nn := types.NamespacedName{
		Namespace: api.PackageNamespace,
		Name:      pbc.Spec.ActiveBundle,
	}

	return nn, nil
}

func (bc *bundleClient) GetBundleList(ctx context.Context, bundles *api.PackageBundleList) error {
	err := bc.Client.List(ctx, bundles, &client.ListOptions{Namespace: api.PackageNamespace})
	if err != nil {
		return fmt.Errorf("listing package bundles: %s", err)
	}
	return nil
}

func (bc *bundleClient) CreateBundle(ctx context.Context, bundle *api.PackageBundle) error {
	err := bc.Client.Create(ctx, bundle)
	if err != nil {
		return fmt.Errorf("creating new package bundle: %s", err)
	}
	return nil
}

func (bc *bundleClient) SaveStatus(ctx context.Context, object client.Object) error {
	return bc.Client.Status().Update(ctx, object, &client.UpdateOptions{})
}

func (bc *bundleClient) Save(ctx context.Context, object client.Object) error {
	return bc.Client.Update(ctx, object, &client.UpdateOptions{})
}
