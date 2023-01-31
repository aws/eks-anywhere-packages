package registrymirror

import (
	"encoding/base64"
	"encoding/json"
	"path"

	corev1 "k8s.io/api/core/v1"

	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/constants"
	k8s "github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/kubernetes"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets/common"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/utils"
)

const (
	endpointKey = "ENDPOINT"
	usernameKey = "USERNAME"
	passwordKey = "PASSWORD"
	caKey       = "CACERTCONTENT"
	insecureKey = "INSECURE"
	credName    = "registry-mirror-cred"
	secretName  = "registry-mirror-secret"
)

type RegistryMirrorSecret struct {
	credName           string
	mgmtClusterName    string
	clientSets         secrets.ClusterClientSet
	clusterCredentials secrets.ClusterCredential
}

var _ secrets.Secret = (*RegistryMirrorSecret)(nil)

func (mirror *RegistryMirrorSecret) Init(mgmtClusterName string, clientSets secrets.ClusterClientSet) error {
	var err error
	mirror.credName = credName
	mirror.mgmtClusterName = mgmtClusterName
	mirror.clientSets = clientSets
	mirror.clusterCredentials, err = mirror.GetClusterCredentials(mirror.clientSets)
	return err
}

func (mirror *RegistryMirrorSecret) IsActive() bool {
	return len(mirror.clusterCredentials) > 0
}

func (mirror *RegistryMirrorSecret) GetClusterCredentials(clientSets secrets.ClusterClientSet) (secrets.ClusterCredential, error) {
	clusterCredentials := make(secrets.ClusterCredential)
	for clusterName, clientSet := range clientSets {
		utils.InfoLogger.Printf("fetching registry mirror auth data for cluster %s...\n", clusterName)
		namespace := constants.PackagesNamespace
		if clusterName != mirror.mgmtClusterName {
			namespace = constants.NamespacePrefix + clusterName
		}
		secret, err := k8s.GetSecret(clientSet, secretName, namespace)
		if err == nil {
			credential := &secrets.Credential{
				Registry: string(secret.Data[endpointKey]),
				Username: string(secret.Data[usernameKey]),
				Password: string(secret.Data[passwordKey]),
				CA:       string(secret.Data[caKey]),
				Insecure: string(secret.Data[insecureKey]),
			}
			if credential.Registry != "" && credential.Username != "" && credential.Password != "" {
				clusterCredentials[clusterName] = []*secrets.Credential{credential}
			}
			utils.InfoLogger.Println("success.")
		} else {
			utils.ErrorLogger.Println(err)
			return nil, err
		}
	}
	return clusterCredentials, nil
}

func (mirror *RegistryMirrorSecret) BroadcastCredentials() error {
	defaultClientSet := mirror.clientSets[mirror.mgmtClusterName]
	data := make(map[string][]byte)
	for clusterName, creds := range mirror.clusterCredentials {
		dockerConfig := common.CreateDockerAuthConfig(creds)
		configJson, err := json.Marshal(*dockerConfig)
		if err != nil {
			return err
		}
		caKey := "ca.crt"
		configKey := "config.json"
		insecureKey := "insecure"
		if clusterName == mirror.mgmtClusterName {
			data[corev1.DockerConfigJsonKey] = configJson
		} else {
			caKey = path.Join(clusterName, caKey)
			configKey = path.Join(clusterName, configKey)
			insecureKey = path.Join(clusterName, insecureKey)
			err = common.BroadcastDockerAuthConfig(dockerConfig, defaultClientSet, mirror.clientSets[clusterName], mirror.credName, clusterName)
			if err != nil {
				return err
			}
		}
		data[caKey] = []byte(creds[0].CA)
		data[configKey] = configJson
		if creds[0].Insecure == base64.StdEncoding.EncodeToString([]byte("false")) {
			data[insecureKey] = []byte(creds[0].Insecure)
		}
	}
	// create a registry mirror secret for package controller pod to mount
	secret, _ := k8s.GetSecret(defaultClientSet, credName, constants.PackagesNamespace)
	if secret == nil {
		_, err := k8s.CreateSecret(defaultClientSet, credName, constants.PackagesNamespace, data)
		if err != nil {
			return err
		}
	} else {
		_, err := k8s.UpdateSecret(defaultClientSet, constants.PackagesNamespace, secret, data)
		if err != nil {
			return err
		}
	}
	return nil
}
