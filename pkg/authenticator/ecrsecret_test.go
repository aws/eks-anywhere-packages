package authenticator

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

func TestAuthFilename(t *testing.T) {
	fakeConfig := rest.Config{}

	t.Run("golden path for set HELM_REGISTRY_CONFIG", func(t *testing.T) {
		testfile := "/test.txt"
		t.Setenv("HELM_REGISTRY_CONFIG", testfile)
		ecrAuth, err := NewECRSecret(&fakeConfig)
		require.NoError(t, err)
		val := ecrAuth.AuthFilename()

		assert.Equal(t, val, testfile)
	})

	t.Run("golden path for no config or secrets", func(t *testing.T) {
		t.Setenv("HELM_REGISTRY_CONFIG", "")
		ecrAuth, _ := NewECRSecret(&fakeConfig)
		val := ecrAuth.AuthFilename()

		assert.Equal(t, val, "")
	})
}

func TestAddToConfigMap(t *testing.T) {
	ctx := context.TODO()
	name := "test-name"
	namespace := "eksa-packages"
	cmdata := make(map[string]string)
	clusterName := "w-test"
	targetClusterNamespace := api.PackageNamespace + "-" + clusterName

	t.Run("golden path for adding new namespace", func(t *testing.T) {
		cmdata["otherns"] = "a"
		mockClientset := fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ConfigMapName,
				Namespace: targetClusterNamespace,
			},
			Data: cmdata,
		})
		ecrAuth := ecrSecret{clientset: mockClientset}
		err := ecrAuth.Initialize(clusterName)
		require.NoError(t, err)

		err = ecrAuth.AddToConfigMap(ctx, name, namespace)
		require.NoError(t, err)

		updatedCM, err := mockClientset.CoreV1().ConfigMaps(targetClusterNamespace).
			Get(ctx, ConfigMapName, metav1.GetOptions{})
		if assert.NoError(t, err) {
			assert.Equal(t, name, updatedCM.Data[namespace])
			assert.Equal(t, "a", updatedCM.Data["otherns"])
		}
	})

	t.Run("golden path for adding one namespace", func(t *testing.T) {
		cmdata[namespace] = "a"
		mockClientset := fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ConfigMapName,
				Namespace: targetClusterNamespace,
			},
			Data: cmdata,
		})
		ecrAuth := ecrSecret{clientset: mockClientset}
		err := ecrAuth.Initialize(clusterName)
		require.NoError(t, err)

		err = ecrAuth.AddToConfigMap(ctx, name, namespace)
		require.NoError(t, err)

		updatedCM, err := mockClientset.CoreV1().ConfigMaps(targetClusterNamespace).
			Get(ctx, ConfigMapName, metav1.GetOptions{})
		if assert.NoError(t, err) {
			assert.ObjectsAreEqual([]string{"a", name},
				strings.Split(updatedCM.Data[namespace], ","))
		}
	})

	t.Run("golden path for not repeating name", func(t *testing.T) {
		name = "a"
		cmdata[namespace] = "a"
		mockClientset := fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ConfigMapName,
				Namespace: targetClusterNamespace,
			},
			Data: cmdata,
		})
		ecrAuth := ecrSecret{clientset: mockClientset}
		err := ecrAuth.Initialize(clusterName)
		require.NoError(t, err)

		err = ecrAuth.AddToConfigMap(ctx, name, namespace)
		require.NoError(t, err)

		updatedCM, _ := mockClientset.CoreV1().ConfigMaps(targetClusterNamespace).
			Get(ctx, ConfigMapName, metav1.GetOptions{})
		assert.Equal(t, "a", updatedCM.Data[namespace])
	})

	t.Run("fails if config map doesnt exist", func(t *testing.T) {
		mockClientset := fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ConfigMapName,
				Namespace: "wrong-ns",
			},
			Data: cmdata,
		})
		ecrAuth := ecrSecret{clientset: mockClientset}
		err := ecrAuth.Initialize(clusterName)
		require.NoError(t, err)

		err = ecrAuth.AddToConfigMap(ctx, name, namespace)

		assert.NotNil(t, err)
	})
}

func TestDelFromConfigMap(t *testing.T) {
	ctx := context.TODO()
	name := "test-name"
	namespace := "eksa-packages"
	cmdata := make(map[string]string)
	clusterName := "w-test"
	targetClusterNamespace := api.PackageNamespace + "-" + clusterName

	t.Run("golden path for removing one name but still exists", func(t *testing.T) {
		name = "a"
		cmdata[namespace] = "a,b"
		mockClientset := fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ConfigMapName,
				Namespace: targetClusterNamespace,
			},
			Data: cmdata,
		})
		ecrAuth := ecrSecret{clientset: mockClientset}
		err := ecrAuth.Initialize(clusterName)
		require.NoError(t, err)

		err = ecrAuth.DelFromConfigMap(ctx, name, namespace)
		require.NoError(t, err)

		updatedCM, err := mockClientset.CoreV1().ConfigMaps(targetClusterNamespace).
			Get(ctx, ConfigMapName, metav1.GetOptions{})
		require.NoError(t, err)

		val, exists := updatedCM.Data["eksa-packages"]
		assert.Nil(t, err)
		assert.True(t, exists)
		assert.Equal(t, "b", val)
	})

	t.Run("golden path for removing one name", func(t *testing.T) {
		name = "a"
		cmdata[namespace] = "a"
		mockClientset := fake.NewSimpleClientset(&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ConfigMapName,
				Namespace: targetClusterNamespace,
			},
			Data: cmdata,
		})
		ecrAuth := ecrSecret{clientset: mockClientset}
		err := ecrAuth.Initialize(clusterName)
		require.NoError(t, err)

		err = ecrAuth.DelFromConfigMap(ctx, name, namespace)
		require.NoError(t, err)
		updatedCM, err := mockClientset.CoreV1().ConfigMaps(targetClusterNamespace).
			Get(ctx, ConfigMapName, metav1.GetOptions{})
		require.NoError(t, err)
		_, exists := updatedCM.Data["eksa-packages"]
		assert.False(t, exists)
	})
}

func TestGetSecretValues(t *testing.T) {
	ctx := context.TODO()
	secretdata := make(map[string][]byte)
	namespace := "eksa-packages"

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
		ecrAuth := ecrSecret{clientset: mockClientset}

		values, err := ecrAuth.GetSecretValues(ctx, namespace)

		assert.Nil(t, err)
		assert.NotNil(t, values["imagePullSecrets"])
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
		ecrAuth := ecrSecret{clientset: mockClientset}

		values, err := ecrAuth.GetSecretValues(ctx, namespace)

		assert.Nil(t, err)
		assert.NotNil(t, values["imagePullSecrets"])

		_, exists := values["pullSecretName"]
		assert.False(t, exists)
		_, exists = values["pullSecretData"]
		assert.False(t, exists)
	})
}

func TestAddSecretToAllNamespace(t *testing.T) {
	ctx := context.TODO()
	suspend := false
	t.Run("golden path for adding ECR Secret to all namespaces", func(t *testing.T) {
		mockClientset := fake.NewSimpleClientset(&batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cronJobName,
				Namespace: api.PackageNamespace,
			},
			Spec: batchv1.CronJobSpec{
				Suspend: &suspend,
			},
		})

		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.AddSecretToAllNamespace(ctx)
		assert.NoError(t, err)
		jobs, err := mockClientset.BatchV1().Jobs(api.PackageNamespace).List(ctx, metav1.ListOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, jobs.Items)
	})

	t.Run("golden path for adding ECR Secret to all namespaces with missing job", func(t *testing.T) {
		mockClientset := fake.NewSimpleClientset(&batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wrong-name",
				Namespace: api.PackageNamespace,
			},
			Spec: batchv1.CronJobSpec{
				Suspend: &suspend,
			},
		})

		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.AddSecretToAllNamespace(ctx)
		assert.NotNil(t, err)
	})

	t.Run("golden path for adding ECR Secret to all namespaces with missing job in namespace", func(t *testing.T) {
		mockClientset := fake.NewSimpleClientset(&batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cronJobName,
				Namespace: "wrong-ns",
			},
			Spec: batchv1.CronJobSpec{
				Suspend: &suspend,
			},
		})

		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.AddSecretToAllNamespace(ctx)
		assert.NotNil(t, err)
	})

	t.Run("golden path for adding ECR Secret to all namespaces skipping if suspended cronjob", func(t *testing.T) {
		suspend = true
		mockClientset := fake.NewSimpleClientset(&batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cronJobName,
				Namespace: api.PackageNamespace,
			},
			Spec: batchv1.CronJobSpec{
				Suspend: &suspend,
			},
		})

		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.AddSecretToAllNamespace(ctx)
		assert.NoError(t, err)
		jobs, err := mockClientset.BatchV1().Jobs(api.PackageNamespace).List(ctx, metav1.ListOptions{})
		assert.NoError(t, err)
		assert.Nil(t, jobs.Items)
	})
}

func TestCleanupPrevRuns(t *testing.T) {
	ctx := context.TODO()
	t.Run("golden path for deleting old auth pods", func(t *testing.T) {
		mockClientset := fake.NewSimpleClientset(&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobExecName + "1",
				Namespace: api.PackageNamespace,
				Labels:    map[string]string{"createdBy": "controller"},
			},
			Status: batchv1.JobStatus{Succeeded: 1},
		})

		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.cleanupPrevRuns(ctx)
		assert.NoError(t, err)
		jobs, err := mockClientset.BatchV1().Jobs(api.PackageNamespace).List(ctx, metav1.ListOptions{})
		assert.NoError(t, err)
		assert.Nil(t, jobs.Items)
	})

	t.Run("golden path for not deleting running auth pods", func(t *testing.T) {
		mockClientset := fake.NewSimpleClientset(&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobExecName + "1",
				Namespace: api.PackageNamespace,
				Labels:    map[string]string{"createdBy": "controller"},
			},
			Status: batchv1.JobStatus{Succeeded: 0},
		})

		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.cleanupPrevRuns(ctx)
		assert.NoError(t, err)
		jobs, err := mockClientset.BatchV1().Jobs(api.PackageNamespace).List(ctx, metav1.ListOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, jobs.Items)
	})

	t.Run("golden path for not deleting jobs not created by controller", func(t *testing.T) {
		mockClientset := fake.NewSimpleClientset(&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobExecName + "1",
				Namespace: api.PackageNamespace,
			},
			Status: batchv1.JobStatus{Succeeded: 1},
		})

		ecrAuth := ecrSecret{clientset: mockClientset}

		err := ecrAuth.cleanupPrevRuns(ctx)
		assert.NoError(t, err)
		jobs, err := mockClientset.BatchV1().Jobs(api.PackageNamespace).List(ctx, metav1.ListOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, jobs.Items)
	})
}
