package authenticator

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CLUSTER_NAME        = "CLUSTER_NAME"
	eksaSystemNamespace = "eksa-system"
)

type TargetClusterClient interface {
	// GetKubeconfigFile for a cluster
	GetKubeconfigFile(ctx context.Context, clusterName string) (fileName string, err error)

	// GetKubeconfigString for a cluster
	GetKubeconfigString(ctx context.Context, clusterName string) (config []byte, err error)
}

type targetClusterClient struct {
	Config *rest.Config
	Client client.Client
}

func NewTargetClusterClient(config *rest.Config, client client.Client) *targetClusterClient {
	return &targetClusterClient{Config: config, Client: client}
}

var _ TargetClusterClient = (*targetClusterClient)(nil)

func (kc *targetClusterClient) getSecretName(clusterName string) string {
	return clusterName + "-kubeconfig"
}

func (kc *targetClusterClient) GetKubeconfigFile(ctx context.Context, clusterName string) (fileName string, err error) {
	kubeconfig, err := kc.GetKubeconfigString(ctx, clusterName)
	if err != nil {
		return "", err
	}
	if len(kubeconfig) == 0 {
		return "", nil
	}

	secretName := kc.getSecretName(clusterName)
	err = os.WriteFile(secretName, kubeconfig, 0600)
	if err != nil {
		return "", fmt.Errorf("opening temporary file: %v", err)
	}

	return secretName, nil
}

func (kc *targetClusterClient) GetKubeconfigString(ctx context.Context, clusterName string) (kubeconfig []byte, err error) {
	// Avoid using kubeconfig for ourselves
	if clusterName == "" || os.Getenv(CLUSTER_NAME) == clusterName {
		// Empty string will cause helm to use the current cluster
		return []byte{}, nil
	}

	secretName := kc.getSecretName(clusterName)
	nn := types.NamespacedName{
		Namespace: eksaSystemNamespace,
		Name:      secretName,
	}
	var kubeconfigSecret corev1.Secret
	if err = kc.Client.Get(ctx, nn, &kubeconfigSecret); err != nil {
		return []byte{}, fmt.Errorf("getting kubeconfig for cluster %q: %w", clusterName, err)
	}

	return kubeconfigSecret.Data["value"], nil
}
