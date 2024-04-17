package registry_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/controllers/mocks"
	"github.com/aws/eks-anywhere-packages/pkg/registry"
)

func TestUpdateIfNeededSuccess(t *testing.T) {
	ctx := context.Background()
	mockClient := mocks.NewMockClient(gomock.NewController(t))
	regSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-mirror-secret",
			Namespace: "eksa-packages-test-cluster",
		},
		Data: map[string][]byte{
			"CACERTCONTENT": bytes.NewBufferString("AAA").Bytes(),
		},
	}
	regCredSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-mirror-cred",
			Namespace: api.PackageNamespace,
		},
		Data: make(map[string][]byte),
	}
	mockClient.EXPECT().Get(ctx, types.NamespacedName{Name: regSecret.Name, Namespace: regSecret.Namespace}, gomock.Any()).
		DoAndReturn(func(ctx context.Context, name types.NamespacedName, s *corev1.Secret, _ ...client.GetOption) error {
			regSecret.DeepCopyInto(s)
			return nil
		})
	mockClient.EXPECT().Get(ctx, types.NamespacedName{Name: regCredSecret.Name, Namespace: regCredSecret.Namespace}, gomock.Any()).
		DoAndReturn(func(ctx context.Context, name types.NamespacedName, s *corev1.Secret, _ ...client.GetOption) error {
			regCredSecret.DeepCopyInto(s)
			return nil
		})
	mockClient.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).Return(nil)

	ci := registry.NewCertInjector(mockClient, logr.Discard())
	err := ci.UpdateIfNeeded(ctx, "test-cluster")
	assert.ErrorIs(t, err, nil)
}

func TestUpdateIfNeededSuccessCertUpdated(t *testing.T) {
	ctx := context.Background()
	mockClient := mocks.NewMockClient(gomock.NewController(t))
	regSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-mirror-secret",
			Namespace: "eksa-packages-test-cluster",
		},
		Data: map[string][]byte{
			"CACERTCONTENT": bytes.NewBufferString("AAA").Bytes(),
		},
	}
	regCredSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-mirror-cred",
			Namespace: api.PackageNamespace,
		},
		Data: map[string][]byte{
			"test-cluster_ca.crt": bytes.NewBufferString("AAA").Bytes(),
		},
	}
	mockClient.EXPECT().Get(ctx, types.NamespacedName{Name: regSecret.Name, Namespace: regSecret.Namespace}, gomock.Any()).
		DoAndReturn(func(ctx context.Context, name types.NamespacedName, s *corev1.Secret, _ ...client.GetOption) error {
			regSecret.DeepCopyInto(s)
			return nil
		})
	mockClient.EXPECT().Get(ctx, types.NamespacedName{Name: regCredSecret.Name, Namespace: regCredSecret.Namespace}, gomock.Any()).
		DoAndReturn(func(ctx context.Context, name types.NamespacedName, s *corev1.Secret, _ ...client.GetOption) error {
			regCredSecret.DeepCopyInto(s)
			return nil
		})

	ci := registry.NewCertInjector(mockClient, logr.Discard())
	err := ci.UpdateIfNeeded(ctx, "test-cluster")
	assert.ErrorIs(t, err, nil)
}

func TestUpdateIfNeededError(t *testing.T) {
	ctx := context.Background()
	mockClient := mocks.NewMockClient(gomock.NewController(t))
	t.Run("registry mirror secret not found", func(t *testing.T) {
		mockClient.EXPECT().Get(ctx, types.NamespacedName{Name: "registry-mirror-secret", Namespace: "eksa-packages-test-cluster"}, gomock.Any()).
			Return(apierrors.NewNotFound(corev1.Resource("secret"), "registry-mirror-secret"))
		ci := registry.NewCertInjector(mockClient, logr.Discard())
		err := ci.UpdateIfNeeded(ctx, "test-cluster")
		assert.ErrorIs(t, err, nil)
	})

	t.Run("registry mirror secret get error", func(t *testing.T) {
		mockClient.EXPECT().Get(ctx, types.NamespacedName{Name: "registry-mirror-secret", Namespace: "eksa-packages-test-cluster"}, gomock.Any()).
			Return(errors.New("get error"))
		ci := registry.NewCertInjector(mockClient, logr.Discard())
		err := ci.UpdateIfNeeded(ctx, "test-cluster")
		assert.ErrorContains(t, err, "fetching CA cert content: getting secret")
	})

	t.Run("registry mirror secret CA content not found", func(t *testing.T) {
		regSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "registry-mirror-secret",
				Namespace: "eksa-packages-test-cluster",
			},
			Data: map[string][]byte{
				"CACERTCONTENT": make([]byte, 0),
			},
		}
		mockClient.EXPECT().Get(ctx, types.NamespacedName{Name: regSecret.Name, Namespace: regSecret.Namespace}, gomock.Any()).
			DoAndReturn(func(ctx context.Context, name types.NamespacedName, s *corev1.Secret, _ ...client.GetOption) error {
				regSecret.DeepCopyInto(s)
				return nil
			})
		ci := registry.NewCertInjector(mockClient, logr.Discard())
		err := ci.UpdateIfNeeded(ctx, "test-cluster")
		assert.ErrorIs(t, err, nil)
	})

	t.Run("registry mirror cred secret get error", func(t *testing.T) {
		regSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "registry-mirror-secret",
				Namespace: "eksa-packages-test-cluster",
			},
			Data: map[string][]byte{
				"CACERTCONTENT": bytes.NewBufferString("AAA").Bytes(),
			},
		}
		mockClient.EXPECT().Get(ctx, types.NamespacedName{Name: regSecret.Name, Namespace: regSecret.Namespace}, gomock.Any()).
			DoAndReturn(func(ctx context.Context, name types.NamespacedName, s *corev1.Secret, _ ...client.GetOption) error {
				regSecret.DeepCopyInto(s)
				return nil
			})
		mockClient.EXPECT().Get(ctx, types.NamespacedName{Name: "registry-mirror-cred", Namespace: "eksa-packages"}, gomock.Any()).
			Return(errors.New("get error"))
		ci := registry.NewCertInjector(mockClient, logr.Discard())
		err := ci.UpdateIfNeeded(ctx, "test-cluster")
		assert.ErrorContains(t, err, "getting secret")
	})
}
