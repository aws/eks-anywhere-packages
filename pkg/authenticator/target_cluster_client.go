package authenticator

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	clusterNameEnvVar   = "CLUSTER_NAME"
	eksaSystemNamespace = "eksa-system"
)

type TargetClusterClient interface {
	// GetKubeconfigFile for a cluster
	GetKubeconfigFile(ctx context.Context, clusterName string) (fileName string, err error)

	// GetKubeconfigString for a cluster
	GetKubeconfigString(ctx context.Context, clusterName string) (config []byte, err error)

	// GetServerVersion of the target api server
	GetServerVersion(ctx context.Context, clusterName string) (info *version.Info, err error)
}

type targetClusterClient struct {
	Config *rest.Config
	Client client.Client
}

func NewTargetClusterClient(config *rest.Config, client client.Client) *targetClusterClient {
	return &targetClusterClient{Config: config, Client: client}
}

var _ TargetClusterClient = (*targetClusterClient)(nil)

func (tcc *targetClusterClient) getSecretName(clusterName string) string {
	return clusterName + "-kubeconfig"
}

func (tcc *targetClusterClient) GetKubeconfigFile(ctx context.Context, clusterName string) (fileName string, err error) {
	kubeconfig, err := tcc.GetKubeconfigString(ctx, clusterName)
	if err != nil {
		return "", err
	}
	if len(kubeconfig) == 0 {
		return "", nil
	}

	secretName := tcc.getSecretName(clusterName)
	err = os.WriteFile(secretName, kubeconfig, 0600)
	if err != nil {
		return "", fmt.Errorf("opening temporary file: %v", err)
	}

	return secretName, nil
}

func (tcc *targetClusterClient) GetKubeconfigString(ctx context.Context, clusterName string) (kubeconfig []byte, err error) {
	// Avoid using kubeconfig for ourselves
	if clusterName == "" || os.Getenv(clusterNameEnvVar) == clusterName {
		// Empty string will cause helm to use the current cluster
		return []byte{}, nil
	}

	secretName := tcc.getSecretName(clusterName)
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
	kubeconfig, err := tcc.GetKubeconfigString(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	config := tcc.Config
	if len(kubeconfig) > 0 {
		clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("creating client config: %s", err)
		}

		config, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("creating rest config config: %s", err)
		}
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating discoveryClient client: %s", err)
	}

	info, err = discoveryClient.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("getting server version: %w", err)
	}
	return info, nil
}
