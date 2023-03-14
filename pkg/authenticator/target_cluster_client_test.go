package authenticator

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/eks-anywhere-packages/controllers/mocks"
)

const actualData = `
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0t
    server: https://127.0.0.1:6443
  name: billy
contexts:
- context:
    cluster: billy
    user: billy-admin
  name: billy-admin@billy
kind: Config
preferences: {}
users:
- name: billy-admin
  user:
    client-certificate-data: LS0t
    client-key-data: LS0t
`

func TestTargetClusterClient_Init(t *testing.T) {
	ctx := context.Background()
	setKubeConfigSecret := func(src *corev1.Secret) func(_ context.Context, _ types.NamespacedName, kc *corev1.Secret, _ ...client.GetOption) error {
		return func(ctx context.Context, name types.NamespacedName, target *corev1.Secret, _ ...client.GetOption) error {
			src.DeepCopyInto(target)
			return nil
		}
	}

	t.Run("get kubeconfig success", func(t *testing.T) {
		logger := testr.New(t)
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		sut := NewTargetClusterClient(logger, nil, mockClient)
		var kubeconfigSecret corev1.Secret
		kubeconfigSecret.Data = make(map[string][]byte)
		kubeconfigSecret.Data["value"] = []byte(actualData)
		nn := types.NamespacedName{
			Namespace: "eksa-system",
			Name:      "billy-kubeconfig",
		}
		mockClient.EXPECT().Get(ctx, nn, gomock.Any()).DoAndReturn(setKubeConfigSecret(&kubeconfigSecret)).Return(nil)
		t.Setenv("CLUSTER_NAME", "franky")

		err := sut.Initialize(ctx, "billy")
		assert.NoError(t, err)
	})

	t.Run("get kubeconfig CLUSTER_NAME success", func(t *testing.T) {
		logger := testr.New(t)
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		sut := NewTargetClusterClient(logger, nil, mockClient)
		t.Setenv("CLUSTER_NAME", "billy")

		err := sut.Initialize(ctx, "billy")
		assert.NoError(t, err)
	})

	t.Run("get kubeconfig failure", func(t *testing.T) {
		logger := testr.New(t)
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		sut := NewTargetClusterClient(logger, nil, mockClient)
		nn := types.NamespacedName{
			Namespace: "eksa-system",
			Name:      "billy-kubeconfig",
		}
		t.Setenv("CLUSTER_NAME", "franky")
		mockClient.EXPECT().Get(ctx, nn, gomock.Any()).Return(fmt.Errorf("boom"))

		err := sut.Initialize(ctx, "billy")
		assert.EqualError(t, err, "getting kubeconfig for cluster \"billy\": boom")
	})

	t.Run("get kubeconfig no cluster", func(t *testing.T) {
		logger := testr.New(t)
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		sut := NewTargetClusterClient(logger, nil, mockClient)

		// TODO do we need to support this case?
		err := sut.Initialize(ctx, "")
		assert.NoError(t, err)
	})
}
