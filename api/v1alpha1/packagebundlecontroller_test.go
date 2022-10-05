package v1alpha1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

const TestBundleName = "v1-21-1003"

func TestPackageBundleController_IsValid(t *testing.T) {
	givenBundleController := func(name string, namespace string) *api.PackageBundleController {
		return &api.PackageBundleController{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
	}

	assert.False(t, givenBundleController("eksa-packaages-bundle-controller", api.PackageNamespace).IsIgnored())
	assert.False(t, givenBundleController("billy", api.PackageNamespace).IsIgnored())
	assert.True(t, givenBundleController("eksa-packages-bundle-controller", "default").IsIgnored())
}

func GivenPackageBundleController() *api.PackageBundleController {
	return &api.PackageBundleController{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eksa-packages-bundle-controller",
			Namespace: api.PackageNamespace,
		},
		Spec: api.PackageBundleControllerSpec{
			ActiveBundle:         TestBundleName,
			DefaultRegistry:      "public.ecr.aws/j0a1m4z9",
			DefaultImageRegistry: "783794618700.dkr.ecr.us-west-2.amazonaws.com",
			BundleRepository:     "eks-anywhere-packages-bundles",
		},
		Status: api.PackageBundleControllerStatus{
			State: api.BundleControllerStateActive,
		},
	}
}

func TestPackageBundleController_GetBundleURI(t *testing.T) {
	sut := GivenPackageBundleController()
	assert.Equal(t, "public.ecr.aws/j0a1m4z9/eks-anywhere-packages-bundles", sut.GetBundleURI())
}

func TestPackageBundleController_GetActiveBundleURI(t *testing.T) {
	sut := GivenPackageBundleController()
	assert.Equal(t, "public.ecr.aws/j0a1m4z9/eks-anywhere-packages-bundles:v1-21-1003", sut.GetActiveBundleURI())
}
