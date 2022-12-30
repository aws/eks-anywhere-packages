package bundle

import (
	"context"
	"fmt"
	"os"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	auth "github.com/aws/eks-anywhere-packages/pkg/authenticator"
)

//go:generate mockgen -source client.go -destination=mocks/client.go -package=mocks Client

type Client interface {
	// GetActiveBundle retrieves the currently active bundle.
	GetActiveBundle(ctx context.Context, clusterName string) (activeBundle *api.PackageBundle, err error)

	// GetPackageBundleController retrieves clusters package bundle controller
	GetPackageBundleController(ctx context.Context, clusterName string) (controller *api.PackageBundleController, err error)

	// GetBundleList get list of bundles worthy of consideration
	GetBundleList(ctx context.Context) (bundles []api.PackageBundle, err error)

	// GetBundle retrieves the named bundle.
	GetBundle(ctx context.Context, name string) (namedBundle *api.PackageBundle, err error)

	// CreateBundle add a new bundle custom resource
	CreateBundle(ctx context.Context, bundle *api.PackageBundle) error

	// CreateClusterNamespace based on cluster name
	CreateClusterNamespace(ctx context.Context, clusterName string) error

	// CreateClusterConfigMap based on cluster name
	CreateClusterConfigMap(ctx context.Context, clusterName string) error

	// CreatePackage creates a package
	CreatePackage(ctx context.Context, pkg *api.Package) (err error)

	// GetPackageList retrieves the list of packages resources.
	GetPackageList(ctx context.Context, namespace string) (packages api.PackageList, err error)

	// SaveStatus saves a resource status
	SaveStatus(ctx context.Context, object client.Object) error

	// Save saves a resource
	Save(ctx context.Context, object client.Object) error
}

type managerClient struct {
	client.Client
}

func NewManagerClient(client client.Client) *managerClient {
	return &(managerClient{
		Client: client,
	})
}

var _ Client = (*managerClient)(nil)

// GetActiveBundle retrieves the bundle from which package are installed.
//
// It retrieves the name of the active bundle from the PackageBundleController,
// then uses the K8s API to retrieve and return the active bundle.
func (bc *managerClient) GetActiveBundle(ctx context.Context, clusterName string) (activeBundle *api.PackageBundle, err error) {
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

func (bc *managerClient) GetPackageBundleController(ctx context.Context, clusterName string) (*api.PackageBundleController, error) {
	if clusterName == "" {
		clusterName = os.Getenv("CLUSTER_NAME")
	}
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

func (bc *managerClient) GetBundle(ctx context.Context, name string) (namedBundle *api.PackageBundle, err error) {
	nn := types.NamespacedName{
		Namespace: api.PackageNamespace,
		Name:      name,
	}
	namedBundle = &api.PackageBundle{}
	err = bc.Get(ctx, nn, namedBundle)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return namedBundle, nil
}

func (bc *managerClient) GetBundleList(ctx context.Context) (bundles []api.PackageBundle, err error) {
	var allBundles = &api.PackageBundleList{}
	err = bc.Client.List(ctx, allBundles, &client.ListOptions{Namespace: api.PackageNamespace})
	if err != nil {
		return nil, fmt.Errorf("listing package bundles: %s", err)
	}

	return allBundles.Items, nil
}

func (bc *managerClient) CreateClusterNamespace(ctx context.Context, clusterName string) error {
	name := api.PackageNamespace + "-" + clusterName
	key := types.NamespacedName{
		Name: name,
	}
	ns := &v1.Namespace{}
	err := bc.Get(ctx, key, ns)
	// Nil err check here means that the namespace exists thus we can just return with no error
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

func (bc *managerClient) CreateClusterConfigMap(ctx context.Context, clusterName string) error {
	name := auth.ConfigMapName
	namespace := api.PackageNamespace + "-" + clusterName
	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	err := bc.Get(ctx, key, cm)
	// Nil err check here means that the config map exists thus we can just return with no error
	if err == nil {
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return err
	}

	cm.Data = make(map[string]string)
	cm.Data[namespace] = "eksa-package-controller"
	// Unfortunate workaround for emissary webhooks hard coded crd namespace
	cm.Data["emissary-system"] = "eksa-package-placeholder"

	err = bc.Client.Create(ctx, cm)
	if err != nil {
		return err
	}
	return nil
}

// CreatePackage Creates the given package resource
func (p *managerClient) CreatePackage(ctx context.Context, pkg *api.Package) (err error) {
	return p.Client.Create(ctx, pkg)
}

// GetPackageList retrieves all packages present in the given namespace
func (p *managerClient) GetPackageList(ctx context.Context, namespace string) (packages api.PackageList, err error) {
	list := api.PackageList{}
	return list, p.Client.List(ctx, &list, client.InNamespace(namespace))
}

func (bc *managerClient) CreateBundle(ctx context.Context, bundle *api.PackageBundle) error {
	err := bc.Client.Create(ctx, bundle)
	if err != nil {
		return fmt.Errorf("creating new package bundle: %s", err)
	}
	return nil
}

func (bc *managerClient) SaveStatus(ctx context.Context, object client.Object) error {
	return bc.Client.Status().Update(ctx, object, &client.UpdateOptions{})
}

func (bc *managerClient) Save(ctx context.Context, object client.Object) error {
	return bc.Client.Update(ctx, object, &client.UpdateOptions{})
}
