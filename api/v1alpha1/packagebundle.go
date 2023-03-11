package v1alpha1

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/version"
)

const (
	PackageBundleKind = "PackageBundle"
	Latest            = "latest"
)

func (config *PackageBundle) MetaKind() string {
	return config.TypeMeta.Kind
}

func (config *PackageBundle) ExpectedKind() string {
	return PackageBundleKind
}

func (config *PackageBundle) FindPackage(pkgName string) (retPkg BundlePackage, err error) {
	for _, pkg := range config.Spec.Packages {
		if strings.EqualFold(pkg.Name, pkgName) {
			return pkg, nil
		}
	}
	return retPkg, fmt.Errorf("package not found in bundle (%s): %s", config.Name, pkgName)
}

func (config *PackageBundle) GetDependencies(version SourceVersion) (dependencies []BundlePackage, err error) {
	for _, dep := range version.Dependencies {
		pkg, err := config.FindPackage(dep)
		if err != nil {
			return nil, err
		}
		dependencies = append(dependencies, pkg)
	}
	return dependencies, nil
}

func (config *PackageBundle) FindVersion(pkg BundlePackage, pkgVersion string) (ret SourceVersion, err error) {
	source := pkg.Source
	for _, packageVersion := range source.Versions {
		// We do not sort before getting `latest` because there will be only a single packageVersion per release in normal cases. For edge cases which may require multiple
		// versions, the order in the file will be ordered according to what we want `latest` to point to
		if packageVersion.Name == pkgVersion || packageVersion.Digest == pkgVersion || pkgVersion == Latest {
			return packageVersion, nil
		}
	}
	return ret, fmt.Errorf("package version not found in bundle (%s): %s @ %s", config.Name, pkg.Name, pkgVersion)
}

func (config *PackageBundle) FindOCISourceByName(pkgName string, pkgVersion string) (retSource PackageOCISource, err error) {
	pkg, err := config.FindPackage(pkgName)
	if err != nil {
		return retSource, err
	}
	return config.FindOCISource(pkg, pkgVersion)
}

func (config *PackageBundle) FindOCISource(pkg BundlePackage, pkgVersion string) (retSource PackageOCISource, err error) {
	packageVersion, err := config.FindVersion(pkg, pkgVersion)
	if err != nil {
		return retSource, err
	}
	return config.GetOCISource(pkg, packageVersion), nil
}

func (config *PackageBundle) GetOCISource(pkg BundlePackage, packageVersion SourceVersion) (retSource PackageOCISource) {
	source := pkg.Source
	return PackageOCISource{Registry: source.Registry, Repository: source.Repository, Digest: packageVersion.Digest, Version: packageVersion.Name}
}

// LessThan evaluates if the left calling bundle is less than the supplied parameter
//
// If the left hand side bundle is older than the right hand side, this
// method returns true. If it is newer (greater) it returns false. If they are
// the same it returns false.
func (config PackageBundle) LessThan(rhsBundle *PackageBundle) bool {
	lhsMajor, lhsMinor, lhsBuild, _ := config.getMajorMinorBuild()
	rhsMajor, rhsMinor, rhsBuild, _ := rhsBundle.getMajorMinorBuild()
	return lhsMajor < rhsMajor || lhsMinor < rhsMinor || lhsBuild < rhsBuild
}

// BundlesByVersion implements sort.Interface for PackageBundles.
type BundlesByVersion []PackageBundle

func (b BundlesByVersion) Len() int {
	return len(b)
}

func (b BundlesByVersion) Less(i, j int) bool {
	return b[i].LessThan(&b[j])
}

func (b BundlesByVersion) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
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
		return major, minor, build, fmt.Errorf("invalid major number <%s>", config.Name)
	} else {
		minor, err = strconv.Atoi(s[1])
		if err != nil {
			return major, minor, build, fmt.Errorf("invalid minor number <%s>", config.Name)
		} else {
			build, err = strconv.Atoi(s[2])
			if err != nil {
				return major, minor, build, fmt.Errorf("invalid build number <%s>", config.Name)
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
func (config *PackageBundle) KubeVersionMatches(targetKubeVersion *version.Info) (matches bool, err error) {
	currKubeMajor, currKubeMinor, _, err := config.getMajorMinorBuild()
	if err != nil {
		return false, err
	}
	return fmt.Sprint(currKubeMajor) == targetKubeVersion.Major && fmt.Sprint(currKubeMinor) == targetKubeVersion.Minor, nil
}

// IsValidVersion returns true if the bundle version is valid
func (config *PackageBundle) IsValidVersion() bool {
	_, _, _, err := config.getMajorMinorBuild()
	return err == nil
}

func (s PackageOCISource) GetChartUri() string {
	return "oci://" + path.Join(s.Registry, s.Repository)
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
	for _, packageVersion := range s.Versions {
		myVersions[packageVersion.Key()] = struct{}{}
	}
	for _, packageVersion := range other.Versions {
		if _, ok := myVersions[packageVersion.Key()]; !ok {
			return false
		}
	}

	otherVersions := make(map[string]struct{})
	for _, packageVersion := range other.Versions {
		otherVersions[packageVersion.Key()] = struct{}{}
	}
	for key := range myVersions {
		if _, ok := otherVersions[key]; !ok {
			return false
		}
	}

	return true
}

func (bp *BundlePackage) GetJsonSchema(pkgVersion *SourceVersion) ([]byte, error) {
	// The package configuration is gzipped and base64 encoded
	// When processing the configuration, the reverse occurs: base64 decode, then unzip
	configuration := pkgVersion.Schema
	decodedConfiguration, err := base64.StdEncoding.DecodeString(configuration)
	if err != nil {
		return nil, fmt.Errorf("error decoding configurations %v", err)
	}

	reader := bytes.NewReader(decodedConfiguration)
	gzreader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("error when uncompressing configurations %v", err)
	}

	output, err := io.ReadAll(gzreader)
	if err != nil {
		return nil, fmt.Errorf("error reading configurations %v", err)
	}

	return output, nil
}

func (v SourceVersion) Key() string {
	return v.Name + " " + v.Digest
}
