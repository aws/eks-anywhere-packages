package v1alpha1

import (
	"fmt"
	"path"
)

type PackageOCISource struct {
	Registry   string `json:"registry"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
}

const (
	PackageBundleKind = "PackageBundle"
)

func (config *PackageBundle) MetaKind() string {
	return config.TypeMeta.Kind
}

func (config *PackageBundle) ExpectedKind() string {
	return PackageBundleKind
}

func (config *PackageBundle) FindSource(pkgName, pkgVersion string) (retSource PackageOCISource, version string, err error) {
	for _, pkg := range config.Spec.Packages {
		if pkg.Name == pkgName {
			source := pkg.Source
			for _, version := range source.Versions {
				if version.Name == pkgVersion || version.Digest == pkgVersion {
					retSource = PackageOCISource{Registry: source.Registry, Repository: source.Repository, Tag: version.Digest}
					return retSource, version.Name, nil
				}
			}
		}
	}

	return retSource, "", fmt.Errorf("package not found: %s @ %s", pkgName, pkgVersion)
}

func (s PackageOCISource) AsRepoURI() string {
	return path.Join(s.Registry, s.Repository)
}

// VersionsMatch returns true if the given source locations match one another.
func (s BundlePackageSource) Matches(other BundlePackageSource) bool {
	if s.Registry != other.Registry {
		return false
	}
	if s.Repository != other.Repository {
		return false
	}

	myVersions := make(map[string]struct{})
	for _, version := range s.Versions {
		myVersions[version.Key()] = struct{}{}
	}
	for _, version := range other.Versions {
		if _, ok := myVersions[version.Key()]; !ok {
			return false
		}
	}

	otherVersions := make(map[string]struct{})
	for _, version := range other.Versions {
		otherVersions[version.Key()] = struct{}{}
	}
	for key := range myVersions {
		if _, ok := otherVersions[key]; !ok {
			return false
		}
	}

	return true
}

func (v SourceVersion) Key() string {
	return v.Name + " " + v.Digest
}
