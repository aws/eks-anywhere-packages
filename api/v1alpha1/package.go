package v1alpha1

import (
	"sigs.k8s.io/yaml"
)

const (
	PackageKind      = "Package"
	PackageNamespace = "eksa-packages"
)

func (config *Package) MetaKind() string {
	return config.TypeMeta.Kind
}

func (config *Package) ExpectedKind() string {
	return PackageKind
}

// GetValues convert spec values into generic values map
func (config *Package) GetValues() (values map[string]interface{}, err error) {
	mapInterfaces := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(config.Spec.Config), &mapInterfaces)
	return mapInterfaces, err
}

func (config *Package) GetFlattenedValues() (values map[string]interface{}, err error) {
	originalValues, err := config.GetValues()
	if err != nil {
		return nil, err
	}
	return config.flatten(originalValues), nil
}

func (config *Package) flatten(originals map[string]interface{}) (values map[string]interface{}) {
	o := make(map[string]interface{})
	for k, v := range originals {
		switch child := v.(type) {
		case map[string]interface{}:
			nm := config.flatten(child)
			for nk, nv := range nm {
				o[k+"."+nk] = nv
			}
		case []interface{}:
			for _, e := range child {
				switch ele := e.(type) {
				case map[string]interface{}:
					nm := config.flatten(ele)
					for nk, nv := range nm {
						o[k+"."+nk] = nv
					}
				}
			}
		default:
			o[k] = v
		}
	}
	return o
}
