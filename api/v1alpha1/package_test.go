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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error unmarshaling")
}

func TestPackage_GetFlattenedValuesSuccess(t *testing.T) {
	provided := `
make: willys
models:
- mb: "41"
- cj2a:
    year: "45"
    day:
    - month: 3
  test: 12
    `
	expected := map[string]interface{}{
		"make":                  "willys",
		"models.mb":             "41",
		"models.cj2a.year":      "45",
		"models.cj2a.day.month": 3.,
		"models.test":           12.,
	}
	ao := givenPackage(provided)

	actual, err := ao.GetFlattenedValues()
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}
