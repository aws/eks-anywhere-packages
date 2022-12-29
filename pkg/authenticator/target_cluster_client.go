package authenticator

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

const (
	clusterNameEnvVar   = "CLUSTER_NAME"
	eksaSystemNamespace = "eksa-system"
)

//go:generate mockgen -source target_cluster_client.go -destination=mocks/target_cluster_client.go -package=mocks TargetClusterClient

type TargetClusterClient interface {
	// Init the target cluster client
	Initialize(ctx context.Context, clusterName string) error

	// GetServerVersion of the target api server
	GetServerVersion(ctx context.Context, clusterName string) (info *version.Info, err error)

	// CreateClusterNamespace for the workload cluster
	CreateClusterNamespace(ctx context.Context, clusterName string) (err error)

	// Implement RESTClientGetter
	ToRESTConfig() (*rest.Config, error)
	ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error)
	ToRESTMapper() (meta.RESTMapper, error)
	ToRawKubeConfigLoader() clientcmd.ClientConfig
}

type targetClusterClient struct {
	Config       *rest.Config
	Client       client.Client
	targetSelf   bool
	clientConfig clientcmd.ClientConfig
}

var _ TargetClusterClient = (*targetClusterClient)(nil)

func NewTargetClusterClient(config *rest.Config, client client.Client) *targetClusterClient {
	return &targetClusterClient{Config: config, Client: client}
}

func (tcc *targetClusterClient) Initialize(ctx context.Context, clusterName string) error {
	kubeconfig, err := tcc.getKubeconfig(ctx, clusterName)
	if err != nil {
		return err
	}

	tcc.targetSelf = false
	if kubeconfig == nil {
		tcc.targetSelf = true
		tcc.clientConfig = clientcmd.NewDefaultClientConfig(clientcmdapi.Config{}, &clientcmd.ConfigOverrides{})
		return nil
	}

	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		return err
	}

	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return err
	}

	tcc.clientConfig = clientcmd.NewDefaultClientConfig(rawConfig, &clientcmd.ConfigOverrides{})
	return nil
}

func (tcc *targetClusterClient) ToRESTConfig() (*rest.Config, error) {
	if tcc.targetSelf {
		return tcc.Config, nil
	}
	return tcc.clientConfig.ClientConfig()
}

func (tcc *targetClusterClient) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	restConfig, err := tcc.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	dc, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return memory.NewMemCacheClient(dc), nil
}

func (tcc *targetClusterClient) ToRESTMapper() (meta.RESTMapper, error) {
	dc, err := tcc.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	return restmapper.NewDeferredDiscoveryRESTMapper(dc), nil
}

func (tcc *targetClusterClient) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return tcc.clientConfig
}

func (tcc *targetClusterClient) getKubeconfig(ctx context.Context, clusterName string) (kubeconfig []byte, err error) {
	// Avoid using kubeconfig for ourselves
	if clusterName == "" || os.Getenv(clusterNameEnvVar) == clusterName {
		// Empty string will cause helm to use the current cluster
		return nil, nil
	}

	secretName := clusterName + "-kubeconfig"
	nn := types.NamespacedName{
		Namespace: eksaSystemNamespace,
		Name:      secretName,
	}
	var kubeconfigSecret corev1.Secret
	if err = tcc.Client.Get(ctx, nn, &kubeconfigSecret); err != nil {
		return []byte{}, fmt.Errorf("getting kubeconfig for cluster %q: %w", clusterName, err)
	}

	return kubeconfigSecret.Data["value"], nil
}

func (tcc *targetClusterClient) GetServerVersion(ctx context.Context, clusterName string) (info *version.Info, err error) {
	err = tcc.Initialize(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("initializing target client: %s", err)
	}

	discoveryClient, err := tcc.ToDiscoveryClient()
	if err != nil {
		return nil, fmt.Errorf("creating discoveryClient client: %s", err)
	}

	info, err = discoveryClient.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("getting server version: %w", err)
	}
	return info, nil
}

func (tcc *targetClusterClient) CreateClusterNamespace(ctx context.Context, clusterName string) (err error) {
	err = tcc.Initialize(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("initializing target client: %s", err)
	}

	restConfig, err := tcc.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("create rest config for %s: %s", clusterName, err)
	}

	k8sClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return fmt.Errorf("creating client for %s: %s", clusterName, err)
	}

	name := api.PackageNamespace
	key := types.NamespacedName{
		Name: name,
	}
	ns := &corev1.Namespace{}
	err = k8sClient.Get(ctx, key, ns)
	// Nil err check here means that the namespace exists thus we can just return with no error
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get namespace for %s: %s", clusterName, err)
	}

	ns.Name = name
	err = k8sClient.Create(ctx, ns)
	if err != nil {
		return fmt.Errorf("create namespace for %s: %s", clusterName, err)
	}
	return nil
}
