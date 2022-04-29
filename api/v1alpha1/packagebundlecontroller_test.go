package v1alpha1_test

import (
	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"testing"

	"github.com/stretchr/testify/assert"
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

	assert.Equal(t, false, givenBundleController(api.PackageBundleControllerName, api.PackageNamespace).IsIgnored())
	assert.Equal(t, true, givenBundleController("billy", api.PackageNamespace).IsIgnored())
	assert.Equal(t, true, givenBundleController(api.PackageBundleControllerName, "default").IsIgnored())
}
