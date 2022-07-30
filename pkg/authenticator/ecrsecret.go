package authenticator

import (
	"context"
	b64 "encoding/base64"
	"os"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	varPackagesNamespace = "eksa-packages"
	varConfigMapName     = "installed-namespaces"
	varConfigMapKey      = "NAMESPACES"
	varECRTokenName      = "ecr-token"
)

type ecrSecret struct {
	clientset kubernetes.Interface
}

var _ Authenticator = (*ecrSecret)(nil)

func NewECRSecret(config *rest.Config) *ecrSecret {
	clientset, _ := kubernetes.NewForConfig(config)

	return &ecrSecret{
		clientset: clientset,
	}
}

func (s *ecrSecret) AuthFilename() (string, error) {
	// Check if Helm registry config is set
	helmconfig := os.Getenv("HELM_REGISTRY_CONFIG")
	if helmconfig != "" {
		// Use HELM_REGISTRY_CONFIG
		return helmconfig, nil
	}

	return "", nil
}

func (s *ecrSecret) UpdateConfigMap(ctx context.Context, namespace string, add bool) error {
	cm, err := s.clientset.CoreV1().ConfigMaps(varPackagesNamespace).
		Get(ctx, varConfigMapName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	addToConfigMap(cm, namespace, add)
	_, err = s.clientset.CoreV1().ConfigMaps(varPackagesNamespace).
		Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func addToConfigMap(cm *v1.ConfigMap, namespace string, add bool) {
	if namespace == "" {
		namespace = "default"
	}
	values := strings.Split(cm.Data[varConfigMapKey], ",")
	if add {
		values = append(values, namespace)
	}

	result := make([]string, 0, len(values))
	check := map[string]bool{}
	for _, val := range values {
		_, ok := check[val]
		if !ok {
			// Ignore namespace if set to remove
			if !add && val == namespace {
				continue
			}
			check[val] = true
			result = append(result, val)
		}
	}

	update := ""
	for _, val := range result {
		if len(update) == 0 {
			update = val
		} else {
			update = update + "," + val
		}
	}

	cm.Data[varConfigMapKey] = update
}

func (s *ecrSecret) GetSecretValues(ctx context.Context) (map[string]interface{}, error) {
	secret, err := s.clientset.CoreV1().Secrets(varPackagesNamespace).Get(ctx, varECRTokenName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	values := make(map[string]interface{})
	var imagePullSecret [1]map[string]string
	imagePullSecret[0] = make(map[string]string)
	imagePullSecret[0]["name"] = varECRTokenName
	values["imagePullSecrets"] = imagePullSecret
	values["pullSecretName"] = varECRTokenName
	values["pullSecretData"] = b64.StdEncoding.EncodeToString(secret.Data[".dockerconfigjson"])
	return values, nil
}
