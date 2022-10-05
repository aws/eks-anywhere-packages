package authenticator

import (
	"context"
	"os"
	"strconv"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/csset"
)

const (
	ConfigMapName = "ns-secret-map"
	ecrTokenName  = "ecr-token"
	cronJobName   = "cron-ecr-renew"
)

type ecrSecret struct {
	clientset     kubernetes.Interface
	targetCluster string
}

var _ Authenticator = (*ecrSecret)(nil)

func NewECRSecret(config *rest.Config) (*ecrSecret, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &ecrSecret{
		clientset:     clientset,
		targetCluster: api.PackageNamespace,
	}, nil
}

func (s *ecrSecret) Initialize(clusterName string) error {
	s.targetCluster = api.PackageNamespace + "-" + clusterName
	// TODO Verify namespace exists
	return nil
}

func (s *ecrSecret) AuthFilename() string {
	// Check if Helm registry config is set
	helmconfig := os.Getenv("HELM_REGISTRY_CONFIG")
	if helmconfig != "" {
		// Use HELM_REGISTRY_CONFIG
		return helmconfig
	}

	return ""
}

func (s *ecrSecret) AddToConfigMap(ctx context.Context, name string, namespace string) error {
	cm, err := s.clientset.CoreV1().ConfigMaps(s.targetCluster).
		Get(ctx, ConfigMapName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	css := csset.NewCSSet(cm.Data[namespace])
	css.Add(name)
	cm.Data[namespace] = css.String()

	_, err = s.clientset.CoreV1().ConfigMaps(s.targetCluster).
		Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (s *ecrSecret) AddSecretToAllNamespace(ctx context.Context) error {
	cronjob, err := s.clientset.BatchV1().CronJobs(api.PackageNamespace).Get(ctx, cronJobName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	jobSpec := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-controller-job-" + strconv.FormatInt(time.Now().UTC().UnixMilli(), 10),
			Namespace: api.PackageNamespace,
		},
		Spec: cronjob.Spec.JobTemplate.Spec,
	}

	jobs := s.clientset.BatchV1().Jobs(api.PackageNamespace)
	_, err = jobs.Create(ctx, jobSpec, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (s *ecrSecret) DelFromConfigMap(ctx context.Context, name string, namespace string) error {
	cm, err := s.clientset.CoreV1().ConfigMaps(s.targetCluster).
		Get(ctx, ConfigMapName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	css := csset.NewCSSet(cm.Data[namespace])
	css.Del(name)
	cm.Data[namespace] = css.String()
	if cm.Data[namespace] == "" {
		delete(cm.Data, namespace)
	}

	_, err = s.clientset.CoreV1().ConfigMaps(s.targetCluster).
		Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (s *ecrSecret) GetSecretValues(ctx context.Context, namespace string) (map[string]interface{}, error) {
	values := make(map[string]interface{})
	var imagePullSecret [1]map[string]string
	imagePullSecret[0] = make(map[string]string)
	imagePullSecret[0]["name"] = ecrTokenName
	values["imagePullSecrets"] = imagePullSecret

	return values, nil
}
