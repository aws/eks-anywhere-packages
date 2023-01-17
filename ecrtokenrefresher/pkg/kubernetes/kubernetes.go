package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/constants"
)

func GetSecret(clientSet kubernetes.Interface, name, namespace string) (*corev1.Secret, error) {
	secret, err := clientSet.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func CreateSecret(clientSet kubernetes.Interface, name string, namespace string, data map[string][]byte) (*corev1.Secret, error) {
	object := metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}
	secret := corev1.Secret{
		ObjectMeta: object,
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       data,
	}
	return clientSet.CoreV1().Secrets(namespace).Create(context.TODO(), &secret, metav1.CreateOptions{})
}

func UpdateSecret(clientSet kubernetes.Interface, namespace string, secret *corev1.Secret, data map[string][]byte) (*corev1.Secret, error) {
	for k, v := range data {
		secret.Data[k] = v
	}
	return clientSet.CoreV1().Secrets(namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
}

func GetNamespaces(clientSet kubernetes.Interface) (*corev1.NamespaceList, error) {
	nslist, err := clientSet.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return nslist, nil
}

func GetConfigMap(clientSet kubernetes.Interface, namespace string) (*corev1.ConfigMap, error) {
	cm, err := clientSet.CoreV1().ConfigMaps(namespace).
		Get(context.TODO(), constants.ConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return cm, err
}
