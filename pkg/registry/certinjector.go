package registry

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

const (
	registryMirrorCredName   = "registry-mirror-cred"
	registryMirrorSecretName = "registry-mirror-secret"
)

type CertInjector struct {
	k8sClient client.Client
	log       logr.Logger
}

// NewCertInjector creates a new CertInjector.
func NewCertInjector(k8sClient client.Client, log logr.Logger) *CertInjector {
	return &CertInjector{
		k8sClient: k8sClient,
		log:       log,
	}
}

// UpdateIfNeeded is responsible for verifying the registry CA cert is available on the mounted `registry-mirror-cred` secret.
func (ci *CertInjector) UpdateIfNeeded(ctx context.Context, clusterName string) error {
	certContent, err := ci.fetchCertContent(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("fetching CA cert content: %v", err)
	}
	if certContent == nil {
		ci.log.Info("No CA cert content found", "cluster", clusterName)
		return nil
	}

	registryMirrorCred := &corev1.Secret{}
	credSecretName := types.NamespacedName{Name: registryMirrorCredName, Namespace: api.PackageNamespace}
	if err := ci.k8sClient.Get(ctx, credSecretName, registryMirrorCred); err != nil {
		return fmt.Errorf("getting secret %s: %s", credSecretName.String(), err)
	}

	credCertKey := fmt.Sprintf("%s_ca.crt", clusterName)
	if _, ok := registryMirrorCred.Data[credCertKey]; !ok {
		ci.log.Info("Updating registry CA cert", "cluster", clusterName, "secret", registryMirrorCredName)
		registryMirrorCred.Data[credCertKey] = certContent
		if err := ci.k8sClient.Update(ctx, registryMirrorCred, &client.UpdateOptions{}); err != nil {
			return fmt.Errorf("updating secret %s: %s", credSecretName.String(), err)
		}
	} else {
		ci.log.Info("CA Cert already updated", "cluster", clusterName)
	}

	return nil
}

// fetchCertContent fetches the CA cert content from `registry-mirror-secret` in the cluster namespace.
// If CACERTCONTENT is empty; we return nil.
// Else we return the contents.
func (ci *CertInjector) fetchCertContent(ctx context.Context, clusterName string) ([]byte, error) {
	managementClusterName := os.Getenv("CLUSTER_NAME")
	registryMirrorSecret := &corev1.Secret{}
	nn := types.NamespacedName{Name: registryMirrorSecretName, Namespace: api.PackageNamespace}
	if clusterName != managementClusterName {
		nn = types.NamespacedName{Name: registryMirrorSecretName, Namespace: fmt.Sprintf("%s-%s", api.PackageNamespace, clusterName)}
	}

	if err := ci.k8sClient.Get(ctx, nn, registryMirrorSecret); err != nil {
		if apierrors.IsNotFound(err) {
			ci.log.Info("Registry mirror secret not found", "error", err)
			return nil, nil
		}
		return nil, fmt.Errorf("getting secret %s: %s", nn.String(), err)
	}

	if len(registryMirrorSecret.Data["CACERTCONTENT"]) > 0 {
		return registryMirrorSecret.Data["CACERTCONTENT"], nil
	}

	return nil, nil
}
