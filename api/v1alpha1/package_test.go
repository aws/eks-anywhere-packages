package v1alpha1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

func givenPackage(config string) api.Package {
	return api.Package{
		Spec: api.PackageSpec{Config: config},
	}
}

func TestPackage_GetValues(t *testing.T) {
	provided := `
make: willys
models:
- mb: "41"
- cj2a:
    year: "45"
  test: 12
    `
	expected := map[string]interface{}{
		"make": "willys",
		"models": []interface{}{
			map[string]interface{}{"mb": "41"},
			map[string]interface{}{
				"cj2a": map[string]interface{}{"year": "45"},
				"test": 12.0,
			},
		},
	}
	ao := givenPackage(provided)

	actual, err := ao.GetValues()
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestPackage_GetValuesError(t *testing.T) {
	provided := `
notactuallyyaml
    `
	ao := givenPackage(provided)
	_, err := ao.GetValues()
	assert.EqualError(t, err, "error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type map[string]interface {}")
	assert.Contains(t, err.Error(), "error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type map[string]interface {}")
}

func TestPackage_GetClusterName(t *testing.T) {
	sut := api.NewPackage("hello-eks-anywhere", "my-hello", "eksa-packages-maggie", "")
	assert.Equal(t, "maggie", sut.GetClusterName())
	sut.Namespace = "eksa-packages"
	assert.Equal(t, "", sut.GetClusterName())
}

func TestPackage_IsOldNamespace(t *testing.T) {
	sut := api.NewPackage("hello-eks-anywhere", "my-hello", "eksa-packages-maggie", "")
	assert.False(t, sut.IsOldNamespace())
	sut.Namespace = "eksa-packages"
	assert.True(t, sut.IsOldNamespace())
}

func TestPackage_IsValidNamespace(t *testing.T) {
	sut := api.NewPackage("hello-eks-anywhere", "my-hello", "eksa-packages-maggie", "")
	assert.True(t, sut.IsValidNamespace())
	sut.Namespace = "eksa-packages"
	assert.True(t, sut.IsValidNamespace())
	sut.Namespace = "default"
	assert.False(t, sut.IsValidNamespace())
}

func TestPackage_IsInstalledOnWorkload(t *testing.T) {
	t.Setenv("CLUSTER_NAME", "maggie")
	sut := api.NewPackage("hello-eks-anywhere", "my-hello", "eksa-packages-maggie", "")
	assert.False(t, sut.IsInstalledOnWorkload())
	sut.Namespace = "eksa-packages"
	assert.True(t, sut.IsInstalledOnWorkload())
	sut.Namespace = "default"
	assert.True(t, sut.IsInstalledOnWorkload())
	sut.Namespace = "eksa-packages-pharrell"
	assert.True(t, sut.IsInstalledOnWorkload())
}
