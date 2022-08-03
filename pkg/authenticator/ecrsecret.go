package authenticator

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"os"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	varPackagesNamespace = "eksa-packages"
	varConfigMapName     = "ns-secret-map"
	varECRTokenName      = "ecr-token"
)

type ecrSecret struct {
	clientset    kubernetes.Interface
	nsReleaseMap map[string]string
}

var _ Authenticator = (*ecrSecret)(nil)

func NewECRSecret(config *rest.Config) (*ecrSecret, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	cm, err := clientset.CoreV1().ConfigMaps(varPackagesNamespace).
		Get(context.TODO(), varConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return &ecrSecret{
		clientset:    clientset,
		nsReleaseMap: cm.Data,
	}, nil
}

func (s *ecrSecret) AuthFilename() string {
	// Check if Helm registry config is set
	helmconfig := os.Getenv("HELM_REGISTRY_CONFIG")
	if helmconfig != "" {
		// Use HELM_REGISTRY_CONFIG
		return helmconfig
	}

	return ""
}

func (s *ecrSecret) UpdateConfigMap(ctx context.Context, name string, namespace string, add bool) error {
	cm, err := s.clientset.CoreV1().ConfigMaps(varPackagesNamespace).
		Get(ctx, varConfigMapName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	addToConfigMap(cm, name, namespace, add)
	_, err = s.clientset.CoreV1().ConfigMaps(varPackagesNamespace).
		Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	// Store data
	s.nsReleaseMap = cm.Data

	return nil
}

func addToConfigMap(cm *v1.ConfigMap, name string, namespace string, add bool) {
	values := strings.Split(cm.Data[namespace], ",")
	if add {
		values = append(values, name)
	}

	result := make([]string, 0, len(values))
	check := map[string]bool{}
	for _, val := range values {
		_, ok := check[val]
		if !ok {
			// Ignore namespace if set to remove
			if !add && val == name {
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

	if update == "" {
		delete(cm.Data, namespace)
	} else {
		cm.Data[namespace] = update
	}
}

func (s *ecrSecret) GetSecretValues(ctx context.Context, namespace string) (map[string]interface{}, error) {
	secret, err := s.clientset.CoreV1().Secrets(varPackagesNamespace).Get(ctx, varECRTokenName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	// Check there is valid token data in the secret
	secretData, exist := secret.Data[".dockerconfigjson"]
	if !exist {
		return nil, fmt.Errorf("No dockerconfigjson data in secret %s", varECRTokenName)
	}

	values := make(map[string]interface{})
	var imagePullSecret [1]map[string]string
	imagePullSecret[0] = make(map[string]string)
	imagePullSecret[0]["name"] = varECRTokenName
	values["imagePullSecrets"] = imagePullSecret

	// if namespace doesn't already have the secret we will fill out the secret.ymal
	if _, exist := s.nsReleaseMap[namespace]; !exist {
		values["pullSecretName"] = varECRTokenName
		values["pullSecretData"] = b64.StdEncoding.EncodeToString(secretData)
	}

	return values, nil
}
