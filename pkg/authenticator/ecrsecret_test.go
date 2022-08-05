package authenticator

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

func TestAuthFilename(t *testing.T) {
	config := rest.Config{}
	t.Run("golden path for set HELM_REGISTRY_CONFIG", func(t *testing.T) {
		testfile := "/test.txt"
		t.Setenv("HELM_REGISTRY_CONFIG", testfile)
		ecrAuth, _ := NewECRSecret(&config)
		val := ecrAuth.AuthFilename()

		assert.Equal(t, val, testfile)
	})

	t.Run("golden path for no config or secrets", func(t *testing.T) {
		t.Setenv("HELM_REGISTRY_CONFIG", "")
		ecrAuth, _ := NewECRSecret(&config)
		val := ecrAuth.AuthFilename()

		assert.Equal(t, val, "")
	})
}

func TestUpdateConfigMap(t *testing.T) {
	ctx := context.TODO()
	name := "test-name"
	namespace := "eksa-packages"
	cmdata := make(map[string]string)

	t.Run("golden path for UpdateConfigMap adding new namespace", func(t *testing.T) {
		operation := ADD
		cmdata["otherns"] = "a"
		mockClientset := fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: api.PackageNamespace,
			},
			Data: cmdata,
		})
		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.UpdateConfigMap(ctx, name, namespace, operation)
		require.NoError(t, err)

		updatedCM, _ := mockClientset.CoreV1().ConfigMaps(api.PackageNamespace).
			Get(ctx, configMapName, metav1.GetOptions{})
		assert.Equal(t, name, updatedCM.Data[namespace])
		assert.Equal(t, "a", updatedCM.Data["otherns"])
	})

	t.Run("golden path for UpdateConfigMap adding one namespace", func(t *testing.T) {
		operation := ADD
		cmdata[namespace] = "a"
		mockClientset := fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: api.PackageNamespace,
			},
			Data: cmdata,
		})
		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.UpdateConfigMap(ctx, name, namespace, operation)
		require.NoError(t, err)

		updatedCM, _ := mockClientset.CoreV1().ConfigMaps(api.PackageNamespace).
			Get(ctx, configMapName, metav1.GetOptions{})
		assert.Equal(t, "a,"+name, updatedCM.Data[namespace])
	})

	t.Run("golden path for UpdateConfigMap not repeating name", func(t *testing.T) {
		operation := ADD
		name = "a"
		cmdata[namespace] = "a"
		mockClientset := fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: api.PackageNamespace,
			},
			Data: cmdata,
		})
		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.UpdateConfigMap(ctx, name, namespace, operation)
		require.NoError(t, err)

		updatedCM, _ := mockClientset.CoreV1().ConfigMaps(api.PackageNamespace).
			Get(ctx, configMapName, metav1.GetOptions{})
		assert.Equal(t, "a", updatedCM.Data[namespace])
	})

	t.Run("golden path for UpdateConfigMap removing one name", func(t *testing.T) {
		operation := REMOVE
		name = "a"
		cmdata[namespace] = "a"
		mockClientset := fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: api.PackageNamespace,
			},
			Data: cmdata,
		})
		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.UpdateConfigMap(ctx, name, namespace, operation)

		updatedCM, _ := mockClientset.CoreV1().ConfigMaps(api.PackageNamespace).
			Get(ctx, configMapName, metav1.GetOptions{})

		_, exists := updatedCM.Data["eksa-packages"]
		assert.Nil(t, err)
		assert.False(t, exists)
	})

	t.Run("golden path for UpdateConfigMap removing one name but still exists", func(t *testing.T) {
		operation := REMOVE
		name = "a"
		cmdata[namespace] = "a,b"
		mockClientset := fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: api.PackageNamespace,
			},
			Data: cmdata,
		})
		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.UpdateConfigMap(ctx, name, namespace, operation)

		updatedCM, _ := mockClientset.CoreV1().ConfigMaps(api.PackageNamespace).
			Get(ctx, configMapName, metav1.GetOptions{})

		val, exists := updatedCM.Data["eksa-packages"]
		assert.Nil(t, err)
		assert.True(t, exists)
		assert.Equal(t, "b", val)
	})

	t.Run("fails if config map doesnt exist", func(t *testing.T) {
		operation := ADD
		mockClientset := fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: "wrong-ns",
			},
			Data: cmdata,
		})
		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.UpdateConfigMap(ctx, name, namespace, operation)

		assert.NotNil(t, err)
	})
}

func TestGetSecretValues(t *testing.T) {
	ctx := context.TODO()
	secretdata := make(map[string][]byte)
	namespace := "eksa-packages"
	releaseMap := make(map[string]string)
	releaseMap[namespace] = "release1"

	t.Run("golden path for Retrieving ECR Secret", func(t *testing.T) {
		namespace = "test"
		testdata := []byte("testdata")
		secretdata[".dockerconfigjson"] = testdata
		mockClientset := fake.NewSimpleClientset(&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ecrTokenName,
				Namespace: api.PackageNamespace,
			},
			Data: secretdata,
			Type: ".dockerconfigjson",
		})
		ecrAuth := ecrSecret{clientset: mockClientset, nsReleaseMap: releaseMap}

		values, err := ecrAuth.GetSecretValues(ctx, namespace)

		assert.Nil(t, err)
		assert.NotNil(t, values["imagePullSecrets"])
		assert.Equal(t, ecrTokenName, values["pullSecretName"])
		assert.Equal(t, base64.StdEncoding.EncodeToString(testdata), values["pullSecretData"])
	})

	t.Run("golden path for Retrieving ECR Secret when namespace already exists", func(t *testing.T) {
		namespace = "eksa-packages"
		testdata := []byte("testdata")
		secretdata[".dockerconfigjson"] = testdata
		mockClientset := fake.NewSimpleClientset(&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ecrTokenName,
				Namespace: api.PackageNamespace,
			},
			Data: secretdata,
			Type: ".dockerconfigjson",
		})
		ecrAuth := ecrSecret{clientset: mockClientset, nsReleaseMap: releaseMap}

		values, err := ecrAuth.GetSecretValues(ctx, namespace)

		assert.Nil(t, err)
		assert.NotNil(t, values["imagePullSecrets"])

		_, exists := values["pullSecretName"]
		assert.False(t, exists)
		_, exists = values["pullSecretData"]
		assert.False(t, exists)
	})

	t.Run("fails when retrieving nonexistant secret", func(t *testing.T) {
		namespace = "eksa-packages"
		testdata := []byte("testdata")
		secretdata[".dockerconfigjson"] = testdata
		mockClientset := fake.NewSimpleClientset(&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ecrTokenName,
				Namespace: "wrong-ns",
			},
			Data: secretdata,
			Type: ".dockerconfigjson",
		})
		ecrAuth := ecrSecret{clientset: mockClientset}

		values, err := ecrAuth.GetSecretValues(ctx, namespace)

		assert.NotNil(t, err)
		assert.Nil(t, values)
	})
}
