package authenticator

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/aws/eks-anywhere-packages/controllers/mocks"
)

func TestTargetClusterClient_GetKubeconfigFile(t *testing.T) {
	ctx := context.Background()
	setKubeConfigSecret := func(src *corev1.Secret) func(_ context.Context, _ types.NamespacedName, kc *corev1.Secret) error {
		return func(ctx context.Context, name types.NamespacedName, target *corev1.Secret) error {
			src.DeepCopyInto(target)
			return nil
		}
	}

	t.Run("get kubeconfig success", func(t *testing.T) {
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		sut := NewTargetClusterClient(nil, mockClient)
		var kubeconfigSecret corev1.Secret
		kubeconfigSecret.Data = make(map[string][]byte)
		kubeconfigSecret.Data["value"] = []byte("actual data")
		nn := types.NamespacedName{
			Namespace: "eksa-system",
			Name:      "billy-kubeconfig",
		}
		mockClient.EXPECT().Get(ctx, nn, gomock.Any()).DoAndReturn(setKubeConfigSecret(&kubeconfigSecret)).Return(nil)
		t.Setenv("CLUSTER_NAME", "franky")

		fileName, err := sut.GetKubeconfigFile(ctx, "billy")
		assert.NoError(t, err)
		assert.Equal(t, "billy-kubeconfig", fileName)
		err = os.Remove(fileName)
		assert.NoError(t, err)
	})

	t.Run("get kubeconfig CLUSTER_NAME success", func(t *testing.T) {
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		sut := NewTargetClusterClient(nil, mockClient)
		t.Setenv("CLUSTER_NAME", "billy")

		fileName, err := sut.GetKubeconfigFile(ctx, "billy")
		assert.NoError(t, err)
		assert.Equal(t, "", fileName)
	})

	t.Run("get kubeconfig failure", func(t *testing.T) {
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		sut := NewTargetClusterClient(nil, mockClient)
		nn := types.NamespacedName{
			Namespace: "eksa-system",
			Name:      "billy-kubeconfig",
		}
		t.Setenv("CLUSTER_NAME", "franky")
		mockClient.EXPECT().Get(ctx, nn, gomock.Any()).Return(fmt.Errorf("boom"))

		fileName, err := sut.GetKubeconfigFile(ctx, "billy")
		assert.EqualError(t, err, "getting kubeconfig for cluster \"billy\": boom")
		assert.Equal(t, "", fileName)
	})

	t.Run("get kubeconfig no cluster", func(t *testing.T) {
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		sut := NewTargetClusterClient(nil, mockClient)

		fileName, err := sut.GetKubeconfigFile(ctx, "")
		assert.NoError(t, err)
		assert.Equal(t, "", fileName)
	})
}
