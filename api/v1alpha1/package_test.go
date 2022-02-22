package v1alpha1_test

import (
	"testing"

	api "github.com/aws/modelrocket-add-ons/api/v1alpha1"
)

func givenPackage(config map[string]string) api.Package {
	return api.Package{
		Spec: api.PackageSpec{Config: config},
	}
}

func TestPackage_GetValues(t *testing.T) {
	expected := map[string]string{"make": "willys", "models.mb": "41", "models.cj2a.year": "45"}
	ao := givenPackage(expected)

	actual, err := ao.GetValues()
	if nil != err {
		t.Errorf("expected <%v> actual <%v>", nil, err)
	}

	var actualValue string
	actualValue = actual["make"].(string)
	if expected["make"] != actualValue {
		t.Errorf("expected <%s> actual <%s>", expected["make"], actualValue)
	}
	actualValue = actual["models"].(map[string]interface{})["mb"].(string)
	if expected["models.mb"] != actualValue {
		t.Errorf("expected <%s> actual <%s>", expected["model.mb"], actualValue)
	}
	actualValue = actual["models"].(map[string]interface{})["cj2a"].(map[string]interface{})["year"].(string)
	if expected["models.cj2a.year"] != actualValue {
		t.Errorf("expected <%s> actual <%s>", expected["model.mb"], actualValue)
	}
}

func TestPackage_GetValuesError(t *testing.T) {
	allExpected := [...]map[string]string{
		{"models.mb": "41", "models.mb.transmission": "T-84"},
		{"models.mb.transmission": "T-84", "models.mb": "41"},
	}
	for _, expected := range allExpected {
		ao := givenPackage(expected)

		expectedError := "key collision models.mb.transmission at mb"
		_, err := ao.GetValues()
		if nil == err {
			t.Errorf("expected <%s> actual <nil>", expectedError)
		}
		if expectedError != err.Error() {
			t.Errorf("expected <%v> actual <%v>", expectedError, err)
		}
	}
}
