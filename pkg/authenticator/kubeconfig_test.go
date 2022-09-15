package authenticator

import (
	"context"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/aws/eks-anywhere-packages/controllers/mocks"
)

func TestManagerContext_getRegistry(t *testing.T) {
	ctx := context.Background()
	setKubeConfigSecret := func(src *corev1.Secret) func(_ context.Context, _ types.NamespacedName, kc *corev1.Secret) error {
		return func(ctx context.Context, name types.NamespacedName, target *corev1.Secret) error {
			src.DeepCopyInto(target)
			return nil
		}
	}

	t.Run("get kubeconfig success", func(t *testing.T) {
		mockClient := mocks.NewMockClient(gomock.NewController(t))
		sut := NewKubeconfigClient(mockClient)
		var kubeconfigSecret corev1.Secret
		nn := types.NamespacedName{
			Namespace: "eksa-system",
			Name:      "billy-kubeconfig",
		}
		mockClient.EXPECT().Get(ctx, nn, gomock.Any()).DoAndReturn(setKubeConfigSecret(&kubeconfigSecret))

		fileName, err := sut.GetKubeconfig(ctx, "billy")
		assert.NoError(t, err)
		assert.Equal(t, "billy-kubeconfig", fileName)
		os.Remove(fileName)
	})
}
