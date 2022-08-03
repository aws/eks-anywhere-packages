package k8s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type config struct {
	Auths map[string]*auth `json:"auths"`
}

type auth struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Auth     string `json:"auth"`
}

const (
	defaultEmail      = "test@test.com"
	configMapName     = "ns-secret-map"
	packagesNamespace = "eksa-packages"
)

func UpdatePasswords(name string, username string, token string, registries string) (error, []string) {
	failedList := make([]string, 0)
	clientset, err := getClientSet()
	if err != nil {
		return err, failedList
	}

	ecrAuth, err := createECRAuthConfig(username, token, registries)
	if err != nil {
		return err, failedList
	}

	targetNamespaces, err := getNamespacesFromConfigMap(clientset)
	if err != nil {
		return err, failedList
	}

	for _, ns := range targetNamespaces {
		secret, err := getSecret(clientset, name, ns)
		if secret == nil {
			secret = createSecret(name, ns)
			secret.Data[corev1.DockerConfigJsonKey] = ecrAuth
			_, err = clientset.CoreV1().Secrets(ns).Create(context.TODO(), secret, metav1.CreateOptions{})
			if err != nil {
				failedList = append(failedList, ns)
			}
		} else {
			secret.Data[corev1.DockerConfigJsonKey] = ecrAuth
			_, err = clientset.CoreV1().Secrets(ns).Update(context.TODO(), secret, metav1.UpdateOptions{})
			if err != nil {
				failedList = append(failedList, ns)
			}
		}
	}

	return err, failedList
}

func getClientSet() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		// Program is not being run from inside cluster. Try default kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("",
			filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		if err != nil {
			return nil, err
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, err
}

func getSecret(clientset *kubernetes.Clientset, name, namespace string) (*corev1.Secret, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func createECRAuthConfig(username, password string, server string) ([]byte, error) {
	config := config{Auths: make(map[string]*auth)}

	config.Auths[server] = &auth{
		Username: username,
		Password: password,
		Email:    defaultEmail,
		Auth:     base64.StdEncoding.EncodeToString([]byte(username + ":" + password)),
	}

	configJson, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	return configJson, nil
}

func createSecret(name string, namespace string) *corev1.Secret {
	object := metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}
	secret := corev1.Secret{
		ObjectMeta: object,
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       map[string][]byte{},
	}
	return &secret
}

func getNamespacesFromConfigMap(clientset *kubernetes.Clientset) ([]string, error) {
	cm, err := clientset.CoreV1().ConfigMaps(packagesNamespace).
		Get(context.TODO(), configMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	values := make([]string, 0)
	for ns := range cm.Data {
		values = append(values, ns)
	}

	return values, err
}
