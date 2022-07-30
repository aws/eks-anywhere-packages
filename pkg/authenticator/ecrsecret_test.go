package authenticator

import (
	"context"
	"encoding/base64"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestAuthFilename(t *testing.T) {
	config := rest.Config{}
	t.Run("golden path for set HELM_REGISTRY_CONFIG", func(t *testing.T) {
		testfile := "/test.txt"
		os.Setenv("HELM_REGISTRY_CONFIG", testfile)
		ecrAuth := NewECRSecret(&config)
		val, err := ecrAuth.AuthFilename()

		assert.Nil(t, err)
		assert.Equal(t, testfile, val)
	})

	t.Run("golden path for no config or secrets", func(t *testing.T) {
		os.Setenv("HELM_REGISTRY_CONFIG", "")
		helmAuth := NewECRSecret(&config)
		val, err := helmAuth.AuthFilename()

		assert.Nil(t, err)
		assert.Equal(t, "", val)
	})
}

func TestUpdateConfigMap(t *testing.T) {
	ctx := context.TODO()
	namespace := "test"
	cmdata := make(map[string]string)

	t.Run("golden path for UpdateConfigMap adding one namespace", func(t *testing.T) {
		add := true
		cmdata[varConfigMapKey] = "eksa-packages"
		mockClientset := fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      varConfigMapName,
				Namespace: varPackagesNamespace,
			},
			Data: cmdata,
		})
		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.UpdateConfigMap(ctx, namespace, add)

		assert.Nil(t, err)

		updatedCM, _ := mockClientset.CoreV1().ConfigMaps(varPackagesNamespace).
			Get(ctx, varConfigMapName, metav1.GetOptions{})
		assert.Equal(t, updatedCM.Data[varConfigMapKey], "eksa-packages"+","+namespace)
	})

	t.Run("golden path for UpdateConfigMap not repeating namespace", func(t *testing.T) {
		add := true
		cmdata[varConfigMapKey] = "eksa-packages,test"
		mockClientset := fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      varConfigMapName,
				Namespace: varPackagesNamespace,
			},
			Data: cmdata,
		})
		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.UpdateConfigMap(ctx, namespace, add)

		assert.Nil(t, err)

		updatedCM, _ := mockClientset.CoreV1().ConfigMaps(varPackagesNamespace).
			Get(ctx, varConfigMapName, metav1.GetOptions{})
		assert.Equal(t, updatedCM.Data[varConfigMapKey], "eksa-packages"+","+namespace)
	})

	t.Run("golden path for UpdateConfigMap removing one namespace", func(t *testing.T) {
		add := false
		cmdata[varConfigMapKey] = "eksa-packages" + "," + namespace
		mockClientset := fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      varConfigMapName,
				Namespace: varPackagesNamespace,
			},
			Data: cmdata,
		})
		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.UpdateConfigMap(ctx, namespace, add)

		updatedCM, _ := mockClientset.CoreV1().ConfigMaps(varPackagesNamespace).
			Get(ctx, varConfigMapName, metav1.GetOptions{})

		assert.Nil(t, err)
		assert.Equal(t, updatedCM.Data[varConfigMapKey], "eksa-packages")
	})

	t.Run("adds default if no namespace provided", func(t *testing.T) {
		add := true
		cmdata[varConfigMapKey] = "eksa-packages"
		namespace = ""
		mockClientset := fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      varConfigMapName,
				Namespace: varPackagesNamespace,
			},
			Data: cmdata,
		})
		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.UpdateConfigMap(ctx, namespace, add)

		assert.Nil(t, err)

		updatedCM, _ := mockClientset.CoreV1().ConfigMaps(varPackagesNamespace).
			Get(ctx, varConfigMapName, metav1.GetOptions{})
		assert.Equal(t, updatedCM.Data[varConfigMapKey], "eksa-packages"+","+"default")
	})

	t.Run("removes default if no namespace provided", func(t *testing.T) {
		add := false
		cmdata[varConfigMapKey] = "eksa-packages,default"
		namespace = ""
		mockClientset := fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      varConfigMapName,
				Namespace: varPackagesNamespace,
			},
			Data: cmdata,
		})
		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.UpdateConfigMap(ctx, namespace, add)

		assert.Nil(t, err)

		updatedCM, _ := mockClientset.CoreV1().ConfigMaps(varPackagesNamespace).
			Get(ctx, varConfigMapName, metav1.GetOptions{})
		assert.Equal(t, updatedCM.Data[varConfigMapKey], "eksa-packages")
	})

	t.Run("fails if config map doesnt exist", func(t *testing.T) {
		add := true
		cmdata[varConfigMapKey] = "eksa-packages,test"
		mockClientset := fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      varConfigMapName,
				Namespace: "wrong-ns",
			},
			Data: cmdata,
		})
		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.UpdateConfigMap(ctx, namespace, add)

		assert.NotNil(t, err)
	})

}

func TestGetSecretValues(t *testing.T) {
	ctx := context.TODO()
	secretdata := make(map[string][]byte)

	t.Run("golden path for Retrieving ECR Secret", func(t *testing.T) {
		testdata := []byte("testdata")
		secretdata[".dockerconfigjson"] = testdata
		mockClientset := fake.NewSimpleClientset(&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      varECRTokenName,
				Namespace: varPackagesNamespace,
			},
			Data: secretdata,
			Type: ".dockerconfigjson",
		})
		ecrAuth := ecrSecret{clientset: mockClientset}

		values, err := ecrAuth.GetSecretValues(ctx)

		assert.Nil(t, err)
		assert.NotNil(t, values["imagePullSecrets"])
		assert.Equal(t, values["pullSecretName"], varECRTokenName)
		assert.Equal(t, values["pullSecretData"], base64.StdEncoding.EncodeToString(testdata))
	})

	t.Run("fails when retrieving nonexistant secret", func(t *testing.T) {
		testdata := []byte("testdata")
		secretdata[".dockerconfigjson"] = testdata
		mockClientset := fake.NewSimpleClientset(&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      varECRTokenName,
				Namespace: "wrong-ns",
			},
			Data: secretdata,
			Type: ".dockerconfigjson",
		})
		ecrAuth := ecrSecret{clientset: mockClientset}

		values, err := ecrAuth.GetSecretValues(ctx)

		assert.NotNil(t, err)
		assert.Nil(t, values)
	})
}
