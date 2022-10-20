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

func (bc *bundleClient) GetBundle(ctx context.Context, name string) (namedBundle *api.PackageBundle, err error) {
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

func (bc *bundleClient) GetBundleList(ctx context.Context) (bundles []api.PackageBundle, err error) {
	var allBundles = &api.PackageBundleList{}
	err = bc.Client.List(ctx, allBundles, &client.ListOptions{Namespace: api.PackageNamespace})
	if err != nil {
		return nil, fmt.Errorf("listing package bundles: %s", err)
	}

	return allBundles.Items, nil
}

func (bc *bundleClient) CreateClusterNamespace(ctx context.Context, clusterName string) error {
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

func (bc *bundleClient) CreateClusterConfigMap(ctx context.Context, clusterName string) error {
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
