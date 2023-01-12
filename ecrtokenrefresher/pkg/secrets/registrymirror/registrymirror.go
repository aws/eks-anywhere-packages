package registrymirror

import (
	"encoding/json"
	"os"

	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/constants"
	k8s "github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/kubernetes"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets/common"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	endpointEnv = "REGISTRY_MIRROR_ENDPOINT"
	usernameEnv = "REGISTRY_MIRROR_USERNAME"
	passwordEnv = "REGISTRY_MIRROR_PASSWORD"
	secretName  = "registry-mirror-cred"
)

type RegistryMirrorSecret struct {
	secretName       string
	defaultClientSet *kubernetes.Clientset
	remoteClientSets secrets.RemoteClusterClientset
}

func (mirror *RegistryMirrorSecret) Init(defaultClientSet *kubernetes.Clientset, remoteClientSets secrets.RemoteClusterClientset) error {
	mirror.secretName = secretName
	mirror.defaultClientSet = defaultClientSet
	mirror.remoteClientSets = remoteClientSets
	return nil
}

func (mirror *RegistryMirrorSecret) IsActive() bool {
	if val, _ := os.LookupEnv(usernameEnv); val == "" {
		return false
	}
	if val, _ := os.LookupEnv(passwordEnv); val == "" {
		return false
	}
	return true
}

func (mirror *RegistryMirrorSecret) GetCredentials() ([]*secrets.Credential, error) {
	utils.InfoLogger.Println("fetching auth data from Registry Mirror... ")
	endpoint, _ := os.LookupEnv(endpointEnv)
	username, _ := os.LookupEnv(usernameEnv)
	password, _ := os.LookupEnv(passwordEnv)
	secrets := []*secrets.Credential{
		{
			Registry: endpoint,
			Username: username,
			Password: password,
		},
	}
	utils.InfoLogger.Println("success.")
	return secrets, nil
}

func (mirror *RegistryMirrorSecret) BroadcastCredentials() error {
	creds, err := mirror.GetCredentials()
	if err != nil {
		return err
	}
	dockerConfig := common.CreateDockerAuthConfig(creds)
	// create a registry mirror secret for package controller pod to mount
	secret, _ := k8s.GetSecret(mirror.defaultClientSet, secretName, constants.PackagesNamespace)
	if secret == nil {
		configJson, err := json.Marshal(*dockerConfig)
		if err != nil {
			return err
		}
		_, err = k8s.CreateSecret(mirror.defaultClientSet, secretName, constants.PackagesNamespace, map[string][]byte{corev1.DockerConfigJsonKey: configJson})
		if err != nil {
			return err
		}
	}
	// create registry mirror secret in all other namespaces where packages get installed
	return common.BroadcastDockerAuthConfig(dockerConfig, &mirror.remoteClientSets, mirror.secretName)
}
