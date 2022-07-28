package v1alpha1

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/ioutil"
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
	lhsMajor, lhsMinor, lhsBuild, _ := config.getMajorMinorBuild()
	rhsMajor, rhsMinor, rhsBuild, _ := rhsBundle.getMajorMinorBuild()
	return lhsMajor < rhsMajor || lhsMinor < rhsMinor || lhsBuild < rhsBuild
}

// getMajorMinorBuild returns the Kubernetes major version, Kubernetes minor
// version, and bundle build version.
func (config *PackageBundle) getMajorMinorBuild() (major int, minor int, build int, err error) {
	s := strings.Split(config.Name, "-")
	s = append(s, "", "", "")
	s[0] = strings.TrimPrefix(s[0], "v")
	build = 0
	minor = 0
	major, err = strconv.Atoi(s[0])
	if err != nil {
		return major, minor, build, fmt.Errorf("inavlid major number <%s>", config.Name)
	} else {
		minor, err = strconv.Atoi(s[1])
		if err != nil {
			return major, minor, build, fmt.Errorf("inavlid minor number <%s>", config.Name)
		} else {
			build, err = strconv.Atoi(s[2])
			if err != nil {
				return major, minor, build, fmt.Errorf("inavlid build number <%s>", config.Name)
			}
		}
	}
	return major, minor, build, err
}

// getMajorMinorFromString returns the Kubernetes major and minor version.
//
// It returns 0, 0 for empty string.
func getMajorMinorFromString(kubeVersion string) (major int, minor int) {
	s := strings.Split(kubeVersion, "-")
	s = append(s, "", "", "")
	s[0] = strings.TrimPrefix(s[0], "v")
	major, _ = strconv.Atoi(s[0])
	minor, _ = strconv.Atoi(s[1])
	return major, minor
}

// KubeVersionMatches returns true if the target Kubernetes matches the
// current bundle's Kubernetes version.
//
// Note the method only compares the major and minor versions of Kubernetes, and
// ignore the patch numbers.
func (config *PackageBundle) KubeVersionMatches(targetKubeVersion string) (matches bool, err error) {
	var currKubeMajor int
	var currKubeMinor int
	currKubeMajor, currKubeMinor, _, err = config.getMajorMinorBuild()
	targetKubeMajor, targetKubeMinor := getMajorMinorFromString(targetKubeVersion)
	if err != nil {
		return false, err
	}
	return currKubeMajor == targetKubeMajor && currKubeMinor == targetKubeMinor, nil
}

func (config *PackageBundle) GetPackageFromBundle(packageName string) (*BundlePackage, error) {
	for _, p := range config.Spec.Packages {
		if p.Name == packageName {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("package %s not found", packageName)
}

func (s PackageOCISource) AsRepoURI() string {
	return path.Join(s.Registry, s.Repository)
}

// PackageMatches returns true if the given source locations match one another.
func (s BundlePackageSource) PackageMatches(other BundlePackageSource) bool {
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

func (bp *BundlePackage) GetJsonSchema() ([]byte, error) {
	// The package configuration is gzipped and base64 encoded
	// When processing the configuration, the reverse occurs: base64 decode, then unzip
	configuration := bp.Source.Versions[0].Schema
	decodedConfiguration, err := base64.StdEncoding.DecodeString(configuration)
	if err != nil {
		return nil, fmt.Errorf("error decoding configurations %v", err)
	}

	reader := bytes.NewReader(decodedConfiguration)
	gzreader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("error when uncompressing configurations %v", err)
	}

	output, err := ioutil.ReadAll(gzreader)
	if err != nil {
		return nil, fmt.Errorf("error reading configurations %v", err)
	}

	return output, nil
}

func (v SourceVersion) Key() string {
	return v.Name + " " + v.Digest
}
