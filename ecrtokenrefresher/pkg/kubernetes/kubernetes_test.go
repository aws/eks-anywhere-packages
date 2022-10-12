package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/aws"
)

func TestPushECRAuthToSecret(t *testing.T) {
	secretname := "ecr-token"
	var creds []aws.ECRAuth
	creds = append(creds, aws.ECRAuth{
		Username: "user",
		Token:    "test",
		Registry: "test@test.com"})
	ecrAuth, err := createECRAuthConfig(creds)
	assert.NoError(t, err)
	targetNamespaces := []string{"ns1", "ns2"}

	t.Run("golden path for creating secret if doesnt exist", func(t *testing.T) {
		mockClientset := fake.NewSimpleClientset()
		failedList := pushECRAuthToSecret(secretname, targetNamespaces, mockClientset, ecrAuth)

		assert.Empty(t, failedList)
		for _, ns := range targetNamespaces {
			secret, err := getSecret(mockClientset, secretname, ns)
			assert.NoError(t, err)

			assert.Equal(t, ecrAuth, secret.Data[v1.DockerConfigJsonKey])
		}
	})

	t.Run("golden path for updating secret if exists", func(t *testing.T) {
		oldSecretData := map[string][]byte{}
		oldSecretData[v1.DockerConfigJsonKey] = []byte{}
		mockClientset := fake.NewSimpleClientset(&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretname,
				Namespace: targetNamespaces[0],
			},
			Data: oldSecretData,
		})

		failedList := pushECRAuthToSecret(secretname, targetNamespaces, mockClientset, ecrAuth)

		assert.Empty(t, failedList)
		for _, ns := range targetNamespaces {
			secret, err := getSecret(mockClientset, secretname, ns)
			assert.NoError(t, err)

			assert.Equal(t, ecrAuth, secret.Data[v1.DockerConfigJsonKey])
		}
	})
}

func TestGetClusterNameFromNamespaces(t *testing.T) {
	mockClientSet := createMockClientsetWithNamespaces()

	t.Run("golden path getting cluster names for namespaces", func(t *testing.T) {
		clusterNames, err := getClusterNameFromNamespaces(mockClientSet)
		assert.NoError(t, err)

		expected := []string{"mgmt", "w1", "w2"}

		assert.ElementsMatch(t, expected, clusterNames)
	})
}

func TestGetTargetNamespacesFromConfigMap(t *testing.T) {
	mockClientSet := createMockClientsetWithConfigMaps()

	t.Run("golden path for getting target namespace from config map", func(t *testing.T) {
		clusterName := "mgmt"
		expected := []string{"test-ns"}

		targetNamespaces, err := getTargetNamespacesFromConfigMap(mockClientSet, clusterName)

		assert.NoError(t, err)
		assert.ElementsMatch(t, expected, targetNamespaces)

	})
}

func createMockClientsetWithConfigMaps() kubernetes.Interface {
	cmdata := make(map[string]string)
	cmdata["test-ns"] = "test-name"
	cmMgmt := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespacePrefix + "mgmt",
		},
		Data: cmdata,
	}

	cmW1 := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespacePrefix + "w1",
		},
		Data: cmdata,
	}

	cmW2 := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespacePrefix + "w2",
		},
		Data: cmdata,
	}

	mockClientset := fake.NewSimpleClientset(cmMgmt, cmW1, cmW2)

	return mockClientset
}

func createMockClientsetWithNamespaces() kubernetes.Interface {
	nsMgmt := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespacePrefix + "mgmt",
		},
	}

	nsW1 := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespacePrefix + "w1",
		},
	}

	nsW2 := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespacePrefix + "w2",
		},
	}

	wrongNs := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bad-name",
		},
	}

	mockClientset := fake.NewSimpleClientset(nsMgmt, nsW1, nsW2, wrongNs)

	return mockClientset
}
