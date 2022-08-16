package bundle

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

type Client interface {
	// GetActiveBundle retrieves the currently active bundle.
	GetActiveBundle(ctx context.Context) (activeBundle *api.PackageBundle, err error)

	// GetActiveBundleNamespacedName retrieves the namespace and name of the
	// currently active bundle.
	GetActiveBundleNamespacedName(ctx context.Context) (types.NamespacedName, error)

	// GetPackageBundleController retrieves clusters package bundle controller
	GetPackageBundleController(ctx context.Context) (controller *api.PackageBundleController, err error)

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

// GetActiveBundle retrieves the bundle from which package are installed.
//
// It retrieves the name of the active bundle from the PackageBundleController,
// then uses the K8s API to retrieve and return the active bundle.
func (bc *bundleClient) GetActiveBundle(ctx context.Context) (activeBundle *api.PackageBundle, err error) {
	pbc, err := bc.GetPackageBundleController(ctx)
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

	return activeBundle, nil
}

func (bc *bundleClient) GetPackageBundleController(ctx context.Context) (*api.PackageBundleController, error) {
	// Get the cluster name
	u := &unstructured.UnstructuredList{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "controlplane.cluster.x-k8s.io",
		Kind:    "KubeadmControlPlane",
		Version: "v1beta1",
	})
	err := bc.List(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("listing KubeadmControlPlane: %v", err)
	}

	kac := u.Items
	name := api.PackageBundleControllerName
	if len(kac) > 0 {
		name = kac[0].GetName()
	}

	pbc := api.PackageBundleController{}
	key := types.NamespacedName{
		Namespace: api.PackageNamespace,
		Name:      name,
	}
	err = bc.Get(ctx, key, &pbc)
	if err != nil {
		return nil, fmt.Errorf("getting PackageBundleController: %v", err)
	}
	return &pbc, nil
}

// GetActiveBundleNamespacedName retrieves the namespace and name of the
// currently active bundle from the PackageBundleController.
func (bc *bundleClient) GetActiveBundleNamespacedName(ctx context.Context) (types.NamespacedName, error) {
	pbc, err := bc.GetPackageBundleController(ctx)
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
