package registrymirror

import (
	"os"

	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets/common"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/utils"
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
	return common.BroadcastDockerAuthConfig(dockerConfig, &mirror.remoteClientSets, mirror.secretName)
}
