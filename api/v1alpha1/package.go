package v1alpha1

import (
	"fmt"
	"sort"
	"strings"
)

const PackageKind = "Package"

func (config *Package) MetaKind() string {
	return config.TypeMeta.Kind
}

func (config *Package) ExpectedKind() string {
	return PackageKind
}

// GetValues convert spec values into generic values map
func (config *Package) GetValues() (values map[string]interface{}, err error) {
	mapInterfaces := make(map[string]interface{})
	keys := make([]string, 0, len(config.Spec.Config))
	for k := range config.Spec.Config {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := config.Spec.Config[k]
		var key string
		subMap := mapInterfaces
		a := strings.Split(k, ".")
		key, a = a[len(a)-1], a[:len(a)-1]
		for _, name := range a {
			if val, ok := subMap[name]; ok {
				if val, ok := val.(map[string]interface{}); ok {
					subMap = val
					continue
				} else {
					return nil, fmt.Errorf("key collision %s at %s", k, name)
				}
			}
			newMap := make(map[string]interface{})
			subMap[name] = newMap
			subMap = newMap
		}
		subMap[key] = v
	}
	return mapInterfaces, nil
}
