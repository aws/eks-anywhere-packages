package authenticator

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const eksaSystemNamespace = "eksa-system"

type KubeconfigClient interface {
	// GetKubeconfig for a cluster
	GetKubeconfig(ctx context.Context, clusterName string) (fileName string, err error)
}

type kubeconfigClient struct {
	Client client.Client
}

func NewKubeconfigClient(client client.Client) *kubeconfigClient {
	return &(kubeconfigClient{Client: client})
}

var _ KubeconfigClient = (*kubeconfigClient)(nil)

func (kc *kubeconfigClient) GetKubeconfig(ctx context.Context, clusterName string) (fileName string, err error) {
	if clusterName == "" {
		// Empty string will cause helm to use the current cluster
		return "", nil
	}

	secretName := clusterName + "-kubeconfig"
	nn := types.NamespacedName{
		Namespace: eksaSystemNamespace,
		Name:      secretName,
	}
	var kubeconfigSecret corev1.Secret
	if err = kc.Client.Get(ctx, nn, &kubeconfigSecret); err != nil {
		return "", fmt.Errorf("getting kubeconfig for cluster %q: %w", clusterName, err)
	}

	if _, err := os.Stat(secretName); err == nil {
		_ = os.Chown(secretName, os.Geteuid(), os.Getegid())
		_ = os.Chmod(secretName, 0600)

	}
	err = os.WriteFile(secretName, kubeconfigSecret.Data["value"], 0600)
	if err != nil {
		return "", fmt.Errorf("opening temporary file: %v", err)
	}

	return secretName, nil
}
