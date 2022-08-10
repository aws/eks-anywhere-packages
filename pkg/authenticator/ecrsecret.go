package authenticator

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

const (
	configMapName = "ns-secret-map"
	ecrTokenName  = "ecr-token"
)

type ecrSecret struct {
	clientset    kubernetes.Interface
	nsReleaseMap map[string]string
}

var _ Authenticator = (*ecrSecret)(nil)

func NewECRSecret(config rest.Interface) (*ecrSecret, error) {
	clientset := kubernetes.New(config)
	nsReleaseMap := make(map[string]string)
	return &ecrSecret{
		clientset:    clientset,
		nsReleaseMap: nsReleaseMap,
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

func (s *ecrSecret) AddToConfigMap(ctx context.Context, name string, namespace string) error {
	cm, err := s.clientset.CoreV1().ConfigMaps(api.PackageNamespace).
		Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	var values []string
	if data, ok := cm.Data[namespace]; ok {
		values = strings.Split(data, ",")
	}
	values = append(values, name)
	result := make([]string, 0, len(values))
	check := map[string]bool{}
	for _, val := range values {
		_, exists := check[val]
		if !exists {
			check[val] = true
			result = append(result, val)
		}
	}

	update := strings.Join(result, ",")
	if update == "" {
		delete(cm.Data, namespace)
	} else {
		cm.Data[namespace] = update
	}
	_, err = s.clientset.CoreV1().ConfigMaps(api.PackageNamespace).
		Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	// Store data
	s.nsReleaseMap = cm.Data

	return nil
}

func (s *ecrSecret) DelFromConfigMap(ctx context.Context, name string, namespace string) error {
	cm, err := s.clientset.CoreV1().ConfigMaps(api.PackageNamespace).
		Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	var values []string
	if data, ok := cm.Data[namespace]; ok {
		values = strings.Split(data, ",")
	}
	result := make([]string, 0, len(values))
	check := map[string]bool{}
	for _, val := range values {
		_, exists := check[val]
		if !exists {
			// Ignore namespace if set to remove
			if val == name {
				continue
			}
			check[val] = true
			result = append(result, val)
		}
	}

	update := strings.Join(result, ",")

	if update == "" {
		delete(cm.Data, namespace)
	} else {
		cm.Data[namespace] = update
	}

	_, err = s.clientset.CoreV1().ConfigMaps(api.PackageNamespace).
		Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	// Store data
	s.nsReleaseMap = cm.Data

	return nil
}

func (s *ecrSecret) GetSecretValues(ctx context.Context, namespace string) (map[string]interface{}, error) {
	secret, err := s.clientset.CoreV1().Secrets(api.PackageNamespace).Get(ctx, ecrTokenName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	// Check there is valid token data in the secret
	secretData, exist := secret.Data[".dockerconfigjson"]
	if !exist {
		return nil, fmt.Errorf("No dockerconfigjson data in secret %s", ecrTokenName)
	}

	values := make(map[string]interface{})
	var imagePullSecret [1]map[string]string
	imagePullSecret[0] = make(map[string]string)
	imagePullSecret[0]["name"] = ecrTokenName
	values["imagePullSecrets"] = imagePullSecret

	// if namespace doesn't already have the secret we will fill out the secret.ymal
	if _, exist := s.nsReleaseMap[namespace]; !exist {
		values["pullSecretName"] = ecrTokenName
		values["pullSecretData"] = b64.StdEncoding.EncodeToString(secretData)
	}

	return values, nil
}
