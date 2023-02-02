package common

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/constants"
	k8s "github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/kubernetes"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/utils"
)

const defaultEmail = "test@test.com"

type dockerConfig struct {
	Auths map[string]*dockerAuth `json:"auths"`
}

type dockerAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Auth     string `json:"auth"`
}

func GetDefaultClientSet() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		// Program is not being run from inside cluster. Try default kubeconfig
		if val, ok := os.LookupEnv("KUBECONFIG"); ok {
			config, err = clientcmd.BuildConfigFromFlags("", val)
		} else {
			config, err = clientcmd.BuildConfigFromFlags("",
				filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		}
		if err != nil {
			return nil, err
		}
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientSet, err
}

func GetRemoteClientSets(defaultClientset *kubernetes.Clientset) (secrets.ClusterClientSet, error) {
	clusterNames, err := getClusterNameFromNamespaces(defaultClientset)
	if err != nil {
		return nil, err
	}

	remoteClientSets := make(secrets.ClusterClientSet)
	for _, clusterName := range clusterNames {
		secretName := clusterName + "-kubeconfig"
		kubeconfigSecret, err := k8s.GetSecret(defaultClientset, secretName, constants.EksaSystemNamespace)
		if err != nil {
			return nil, err
		}

		kubeconfig := kubeconfigSecret.Data["value"]
		if len(kubeconfig) <= 0 {
			return nil, fmt.Errorf("kubeconfig string in secret not set")
		}

		clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("creating client config: %s", err)
		}

		config, err := clientConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("creating rest config config: %s", err)
		}

		remoteClientSet, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, err
		}
		remoteClientSets[clusterName] = remoteClientSet
	}
	return remoteClientSets, nil
}

func getClusterNameFromNamespaces(clientSet kubernetes.Interface) ([]string, error) {
	clusterNameList := make([]string, 0)
	nslist, err := k8s.GetNamespaces(clientSet)
	if err != nil {
		return nil, err
	}

	for _, ns := range nslist.Items {
		if strings.HasPrefix(ns.Name, constants.NamespacePrefix) {
			clusterName := strings.TrimPrefix(ns.Name, constants.NamespacePrefix)
			clusterNameList = append(clusterNameList, clusterName)
		}
	}

	return clusterNameList, nil
}

func CreateDockerAuthConfig(creds []*secrets.Credential) *dockerConfig {
	config := dockerConfig{Auths: make(map[string]*dockerAuth)}

	for _, cred := range creds {
		config.Auths[cred.Registry] = &dockerAuth{
			Username: cred.Username,
			Password: cred.Password,
			Email:    defaultEmail,
			Auth:     base64.StdEncoding.EncodeToString([]byte(cred.Username + ":" + cred.Password)),
		}
	}

	return &config
}

func BroadcastDockerAuthConfig(configJson []byte, defaultClientSet, remoteClientSet kubernetes.Interface, secretName, clusterName string) {
	namespaces, err := getNamespacesFromConfigMap(defaultClientSet, constants.NamespacePrefix+clusterName)
	if err != nil {
		utils.WarningLogger.Printf("failed to find config map for cluster %s\n", clusterName)
		return
	}
	for _, ns := range namespaces {
		secret, _ := k8s.GetSecret(remoteClientSet, secretName, ns)
		if secret == nil {
			_, err = k8s.CreateSecret(remoteClientSet, secretName, ns, map[string][]byte{corev1.DockerConfigJsonKey: configJson})
			if err != nil {
				utils.WarningLogger.Printf("failed to create %s in %s namespace\n", secretName, ns)
			}
		} else {
			_, err = k8s.UpdateSecret(remoteClientSet, ns, secret, map[string][]byte{corev1.DockerConfigJsonKey: configJson})
			if err != nil {
				utils.WarningLogger.Printf("failed to update %s in %s namespace\n", secretName, ns)
			}
		}
	}
}

func getNamespacesFromConfigMap(clientSet kubernetes.Interface, namespace string) ([]string, error) {
	cm, err := k8s.GetConfigMap(clientSet, namespace)
	if err != nil {
		return nil, err
	}

	values := make([]string, 0)
	for ns := range cm.Data {
		values = append(values, ns)
	}

	return values, err
}
