package v1alpha1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

func TestPackageBundleController_IsValid(t *testing.T) {
	givenBundleController := func(name string, namespace string) *api.PackageBundleController {
		return &api.PackageBundleController{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
	}

	assert.False(t, givenBundleController(api.PackageBundleControllerName, api.PackageNamespace).IsIgnored())
	assert.True(t, givenBundleController("billy", api.PackageNamespace).IsIgnored())
	assert.True(t, givenBundleController(api.PackageBundleControllerName, "default").IsIgnored())
}

func TestPackageBundleControllerSource_GetRef(t *testing.T) {
	sut := api.PackageBundleControllerSource{
		Registry:   "public.ecr.aws/l0g8r8j6",
		Repository: "eks-anywhere-packages-bundles",
	}
	assert.Equal(t, "public.ecr.aws/l0g8r8j6/eks-anywhere-packages-bundles", sut.GetRef())
}
