package bundle

import (
	"context"
	"fmt"
	"sort"
	"strings"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

type Client interface {
	// GetActiveBundle retrieves the currently active bundle.
	GetActiveBundle(ctx context.Context, clusterName string) (activeBundle *api.PackageBundle, err error)

	// GetPackageBundleController retrieves clusters package bundle controller
	GetPackageBundleController(ctx context.Context, clusterName string) (controller *api.PackageBundleController, err error)

	// GetBundleList get list of bundles worthy of consideration
	GetBundleList(ctx context.Context, serverVersion string) (bundles []api.PackageBundle, err error)

	// CreateBundle add a new bundle custom resource
	CreateBundle(ctx context.Context, bundle *api.PackageBundle) error

	// CreateClusterNamespace based on cluster name
	CreateClusterNamespace(ctx context.Context, clusterName string) error

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
func (bc *bundleClient) GetActiveBundle(ctx context.Context, clusterName string) (activeBundle *api.PackageBundle, err error) {
	pbc, err := bc.GetPackageBundleController(ctx, clusterName)
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

func (bc *bundleClient) GetPackageBundleController(ctx context.Context, clusterName string) (*api.PackageBundleController, error) {
	pbc := api.PackageBundleController{}
	key := types.NamespacedName{
		Namespace: api.PackageNamespace,
		Name:      clusterName,
	}
	err := bc.Get(ctx, key, &pbc)
	if err != nil {
		return nil, fmt.Errorf("getting PackageBundleController: %v", err)
	}
	return &pbc, nil
}

func (bc *bundleClient) GetBundleList(ctx context.Context, serverVersion string) (bundles []api.PackageBundle, err error) {
	var allBundles = &api.PackageBundleList{}
	err = bc.Client.List(ctx, allBundles, &client.ListOptions{Namespace: api.PackageNamespace})
	if err != nil {
		return nil, fmt.Errorf("listing package bundles: %s", err)
	}

	sortedBundles := allBundles.Items
	sortFn := func(i, j int) bool {
		if strings.HasPrefix(sortedBundles[i].Name, serverVersion) {
			if !strings.HasPrefix(sortedBundles[j].Name, serverVersion) {
				return true
			}
		} else if strings.HasPrefix(sortedBundles[j].Name, serverVersion) {
			return false
		}

		return sortedBundles[j].LessThan(&sortedBundles[i])
	}
	sort.Slice(sortedBundles, sortFn)

	return sortedBundles, nil
}

func (bc *bundleClient) CreateClusterNamespace(ctx context.Context, clusterName string) error {
	name := api.PackageNamespace + "-" + clusterName
	key := types.NamespacedName{
		Name: name,
	}
	ns := &v1.Namespace{}
	err := bc.Get(ctx, key, ns)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	ns.Name = name
	err = bc.Client.Create(ctx, ns)
	if err != nil {
		return err
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
