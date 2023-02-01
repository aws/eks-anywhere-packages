package common

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/constants"
	k8s "github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/kubernetes"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets"
)

const (
	secretName  = "test-secret"
	clusterName = "test-cluster"
)

var (
	ns = &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: constants.NamespacePrefix + clusterName,
		},
	}
	cm = &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.ConfigMapName,
			Namespace: constants.NamespacePrefix + clusterName,
		},
		Data: map[string]string{
			"ns1": secretName,
			"ns2": secretName,
		},
	}
	secret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "ns1",
		},
		Data: map[string][]byte{
			v1.DockerConfigJsonKey: {},
		},
	}
)

func TestCreateDockerAuthConfig(t *testing.T) {
	creds := []*secrets.Credential{
		{
			Username: "user",
			Password: "password",
			Registry: "1.2.3.4",
		},
	}
	dockerConfig := CreateDockerAuthConfig(creds)
	assert.NotNil(t, dockerConfig)
	assert.Equal(t, len(dockerConfig.Auths), 1)
	assert.EqualValues(t, &dockerAuth{
		Username: "user",
		Password: "password",
		Email:    defaultEmail,
		Auth:     base64.StdEncoding.EncodeToString([]byte("user:password")),
	}, dockerConfig.Auths["1.2.3.4"])
}

func TestBroadcastDockerAuthConfig(t *testing.T) {
	dockerConfig := dockerConfig{
		Auths: map[string]*dockerAuth{
			"1.2.3.4": {
				Username: "user",
				Password: "password",
				Email:    "test@test.com",
				Auth:     "dXNlcjpwYXNzd29yZA==",
			},
		},
	}
	configJson, _ := json.Marshal(dockerConfig)
	defaultClientSet := fake.NewSimpleClientset(cm)
	t.Run("create secret", func(t *testing.T) {
		remoteClientSets := secrets.ClusterClientSet{
			clusterName: fake.NewSimpleClientset(),
		}
		BroadcastDockerAuthConfig(configJson, defaultClientSet, remoteClientSets[clusterName], secretName, clusterName)

		for _, ns := range []string{"ns1", "ns2"} {
			secret, err := k8s.GetSecret(remoteClientSets[clusterName], secretName, ns)
			assert.NoError(t, err)
			assert.Equal(t, configJson, secret.Data[v1.DockerConfigJsonKey])
		}
	})

	t.Run("update secret", func(t *testing.T) {
		remoteClientSets := secrets.ClusterClientSet{
			clusterName: fake.NewSimpleClientset(secret),
		}
		BroadcastDockerAuthConfig(configJson, defaultClientSet, remoteClientSets[clusterName], secretName, clusterName)

		secret, err := k8s.GetSecret(remoteClientSets[clusterName], secretName, "ns1")
		assert.NoError(t, err)
		assert.Equal(t, configJson, secret.Data[v1.DockerConfigJsonKey])
	})
}

func TestGetClusterNameFromNamespaces(t *testing.T) {
	clientSet := fake.NewSimpleClientset(ns)
	nsList, err := getClusterNameFromNamespaces(clientSet)
	assert.NoError(t, err)
	assert.ElementsMatch(t, nsList, []string{clusterName})
}

func TestGetNamespacesFromConfigMap(t *testing.T) {
	clientSet := fake.NewSimpleClientset(cm)
	nsList, err := getNamespacesFromConfigMap(clientSet, constants.NamespacePrefix+clusterName)
	assert.NoError(t, err)
	assert.ElementsMatch(t, nsList, []string{"ns1", "ns2"})
}
