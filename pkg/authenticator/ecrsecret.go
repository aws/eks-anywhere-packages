package authenticator

import (
	"context"
	"os"
	"strconv"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/csset"
)

const (
	ConfigMapName  = "ns-secret-map"
	ecrTokenName   = "ecr-token"
	cronJobName    = "cron-ecr-renew"
	jobExecName    = "eksa-auth-refresher-"
	MirrorCredName = "registry-mirror-cred"
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

func (s *ecrSecret) AuthFilename() string {
	// Check if Helm registry config is set
	helmconfig := os.Getenv("HELM_REGISTRY_CONFIG")
	if helmconfig != "" {
		// Use HELM_REGISTRY_CONFIG
		return helmconfig
	}

	return ""
}

func (s *ecrSecret) Initialize(clusterName string) error {
	s.targetCluster = api.PackageNamespace + "-" + clusterName
	return nil
}

func (s *ecrSecret) AddToConfigMap(ctx context.Context, name, namespace string) error {
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

	if *cronjob.Spec.Suspend {
		return nil
	}

	jobSpec := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobExecName + strconv.FormatInt(time.Now().UTC().UnixMilli(), 10),
			Namespace: api.PackageNamespace,
			Labels:    map[string]string{"createdBy": "controller"},
		},
		Spec: cronjob.Spec.JobTemplate.Spec,
	}

	jobs := s.clientset.BatchV1().Jobs(api.PackageNamespace)
	_, err = jobs.Create(ctx, jobSpec, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	err = s.cleanupPrevRuns(ctx)
	return err
}

func (s *ecrSecret) DelFromConfigMap(ctx context.Context, name, namespace string) error {
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
	values["imagePullSecrets"] = []interface{}{
		map[string]interface{}{"name": ecrTokenName},
		map[string]interface{}{"name": MirrorCredName},
	}

	return values, nil
}

func (s *ecrSecret) cleanupPrevRuns(ctx context.Context) error {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"createdBy": "controller"}}
	deletePropagation := metav1.DeletePropagationBackground
	jobs, err := s.clientset.BatchV1().Jobs(api.PackageNamespace).
		List(ctx, metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
	if err != nil {
		return err
	}
	for _, job := range jobs.Items {
		if job.Status.Succeeded == 1 {
			err := s.clientset.BatchV1().Jobs(api.PackageNamespace).
				Delete(ctx, job.Name,
					metav1.DeleteOptions{PropagationPolicy: &deletePropagation})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
