package v1alpha1

import (
	"fmt"
	"path"
	"strconv"
	"strings"
)

type PackageOCISource struct {
	Version    string `json:"version"`
	Registry   string `json:"registry"`
	Repository string `json:"repository"`
	Digest     string `json:"digest"`
}

const (
	PackageBundleKind           = "PackageBundle"
	PackageBundleControllerName = "eksa-packages-bundle-controller"
	Latest                      = "latest"
)

func (config *PackageBundle) MetaKind() string {
	return config.TypeMeta.Kind
}

func (config *PackageBundle) ExpectedKind() string {
	return PackageBundleKind
}

func (config *PackageBundle) FindSource(pkgName, pkgVersion string) (retSource PackageOCISource, err error) {
	for _, pkg := range config.Spec.Packages {
		if strings.EqualFold(pkg.Name, pkgName) {
			source := pkg.Source
			for _, version := range source.Versions {
				//We do not sort before getting `latest` because there will be only a single version per release in normal cases. For edge cases which may require multiple
				//versions, the order in the file will be ordered according to what we want `latest` to point to
				if version.Name == pkgVersion || version.Digest == pkgVersion || pkgVersion == Latest {
					retSource = PackageOCISource{Registry: source.Registry, Repository: source.Repository, Digest: version.Digest, Version: version.Name}
					return retSource, nil
				}
			}
		}
	}

	return retSource, fmt.Errorf("package not found in bundle (%s): %s @ %s", config.ObjectMeta.Name, pkgName, pkgVersion)
}

// LessThan evaluates if the left calling bundle is less than the supplied parameter
//
// If the left hand side bundle is older than the right hand side, this
// method returns true. If it is newer (greater) it returns false. If they are
// the same it returns false.
func (config *PackageBundle) LessThan(rhsBundle *PackageBundle) bool {
	lhsMajor, lhsMinor, lhsBuild := config.GetMajorMinorBuild()
	rhsMajor, rhsMinor, rhsBuild := rhsBundle.GetMajorMinorBuild()
	return lhsMajor < rhsMajor || lhsMinor < rhsMinor || lhsBuild < rhsBuild
}

func (config *PackageBundle) GetMajorMinorBuild() (major int, minor int, build int) {
	s := strings.Split(config.Name, "-")
	s = append(s, "", "", "")
	s[0] = strings.TrimPrefix(s[0], "v")
	major, _ = strconv.Atoi(s[0])
	minor, _ = strconv.Atoi(s[1])
	build, _ = strconv.Atoi(s[2])
	return major, minor, build
}

func (s PackageOCISource) AsRepoURI() string {
	return path.Join(s.Registry, s.Repository)
}

// Matches returns true if the given source locations match one another.
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
