package k8s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	defaultEmail        = "test@test.com"
	configMapName       = "ns-secret-map"
	eksaSystemNamespace = "eksa-system"
	packagesNamespace   = "eksa-packages"
	namespacePrefix     = packagesNamespace + "-"
)

func UpdateTokens(secretname string, username string, token string, registries string) (error, []string) {
	failedList := make([]string, 0)
	clientset, err := getDefaultClientSet()
	if err != nil {
		return err, failedList
	}

	ecrAuth, err := createECRAuthConfig(username, token, registries)
	if err != nil {
		return err, failedList
	}

	clusterNames, err := getClusterNameFromNamespaces(clientset)
	if err != nil {
		return err, failedList
	}

	for _, clusterName := range clusterNames {
		targetNamespaces, err := getTargetNamespacesFromConfigMap(clientset, clusterName)
		if err != nil {
			failedList = append(failedList, fmt.Sprintf("Failed to find config map for %s cluster, ", clusterName))
			continue
		}

		remoteClientset, err := getRemoteClient(clusterName, clientset)
		if err != nil {
			failedList = append(failedList, fmt.Sprintf("Failed to create client for %s cluster, ", clusterName))
			continue
		}

		failedList = append(failedList, pushECRAuthToSecret(secretname, targetNamespaces, remoteClientset, ecrAuth)...)
	}

	return nil, failedList
}

func getRemoteClient(clusterName string, defaultClientset *kubernetes.Clientset) (*kubernetes.Clientset, error) {
	secretName := clusterName + "-kubeconfig"
	kubeconfigSecret, err := getSecret(defaultClientset, secretName, eksaSystemNamespace)
	if err != nil {
		return nil, err
	}
	kubeconfig := kubeconfigSecret.Data["value"]

	if len(kubeconfig) <= 0 {
		return nil, fmt.Errorf("Kubeconfig string in secret not set")
	}

	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("creating client config: %s", err)
	}

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("creating rest config config: %s", err)
	}

	remoteClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return remoteClient, nil
}

func pushECRAuthToSecret(secretname string, targetNamespaces []string, clientset kubernetes.Interface, ecrAuth []byte) []string {
	failedList := make([]string, 0)
	for _, ns := range targetNamespaces {
		secret, err := getSecret(clientset, secretname, ns)
		if secret == nil {
			secret = createSecret(secretname, ns)
			secret.Data[corev1.DockerConfigJsonKey] = ecrAuth
			_, err = clientset.CoreV1().Secrets(ns).Create(context.TODO(), secret, metav1.CreateOptions{})
			if err != nil {
				failedList = append(failedList, fmt.Sprintf("Failed to create %s in %s namespace, ", secretname, ns))
			}
		} else {
			secret.Data[corev1.DockerConfigJsonKey] = ecrAuth
			_, err = clientset.CoreV1().Secrets(ns).Update(context.TODO(), secret, metav1.UpdateOptions{})
			if err != nil {
				failedList = append(failedList, fmt.Sprintf("Failed to update %s in %s namespace, ", secretname, ns))
			}
		}
	}
	return failedList
}

func getDefaultClientSet() (*kubernetes.Clientset, error) {
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

func getSecret(clientset kubernetes.Interface, name, namespace string) (*corev1.Secret, error) {
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

func getClusterNameFromNamespaces(clientset kubernetes.Interface) ([]string, error) {
	clusterNameList := make([]string, 0)
	nslist, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, ns := range nslist.Items {
		if strings.HasPrefix(ns.Name, namespacePrefix) {
			clusterName := strings.TrimPrefix(ns.Name, namespacePrefix)
			clusterNameList = append(clusterNameList, clusterName)
		}
	}

	return clusterNameList, nil
}

func getTargetNamespacesFromConfigMap(clientset kubernetes.Interface, clusterName string) ([]string, error) {
	cm, err := clientset.CoreV1().ConfigMaps(namespacePrefix+clusterName).
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
