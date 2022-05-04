package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

const (
	YamlSeparator = "\n---\n"
)

func ValidateInputConfig(fileName string) (*Input, error) {
	inputconfig := &Input{}
	err := ParseInputConfig(fileName, inputconfig)
	if err != nil {
		return nil, err
	}
	err = ValidateInputConfigContent(inputconfig)
	if err != nil {
		return nil, err
	}
	return inputconfig, err
}

func ValidateInputConfigContent(inputConfig *Input) error {
	for _, v := range inputConfigValidations {
		if err := v(inputConfig); err != nil {
			return err
		}
	}
	return nil
}

var inputConfigValidations = []func(*Input) error{
	validateInputConfigName,
}

func validateInputConfigName(inputConfig *Input) error {
	err := inputConfig.ValidateInputNotEmpty()
	if err != nil {
		return err
	}
	return nil
}

func (inputConfig *Input) ValidateInputNotEmpty() error {
	// Check if Projects are listed
	if len(inputConfig.Packages) < 1 {
		return fmt.Errorf("Should use non-empty list of projects for input")
	}
	return nil
}

func ValidateBundle(fileName string) (*api.PackageBundle, error) {
	bundle, err := ParseBundle(fileName)
	if err != nil {
		return nil, err
	}
	err = ValidateBundleContent(bundle)
	if err != nil {
		return nil, err
	}
	return bundle, err
}

func ValidateBundleContent(bundle *api.PackageBundle) error {
	for _, v := range bundleValidations {
		if err := v(bundle); err != nil {
			return err
		}
	}
	return nil
}

var bundleValidations = []func(*api.PackageBundle) error{
	validateBundleName,
}

func validateBundleName(bundle *api.PackageBundle) error {
	err := ValidateBundleNoSignature(bundle)
	if err != nil {
		return err
	}
	return nil
}

func ValidateBundleNoSignature(bundle *api.PackageBundle) error {
	// Check if Projects are listed
	if len(bundle.Spec.Packages) < 1 {
		return fmt.Errorf("Should use non-empty list of projects for input")
	}
	return nil
}

func ParseBundle(filename string) (*api.PackageBundle, error) {
	content, err := ioutil.ReadFile(filepath.Clean(filename))
	if err != nil {
		return nil, fmt.Errorf("reading package bundle file %q: %w", filename, err)
	}

	for _, doc := range bytes.Split(content, []byte(YamlSeparator)) {
		bundle := &api.PackageBundle{}
		err = yaml.UnmarshalStrict(doc, bundle)
		if err != nil {
			return nil, fmt.Errorf("unmarshaling package bundle from %q: %w",
				filename, err)
		}
		return bundle, nil
	}

	return nil, fmt.Errorf("invalid package bundle file %q", filename)
}

func ParseInputConfig(fileName string, inputConfig *Input) error {
	content, err := ioutil.ReadFile(filepath.Clean(fileName))
	if err != nil {
		return fmt.Errorf("unable to read file due to: %v", err)
	}
	for _, c := range strings.Split(string(content), YamlSeparator) {
		if err = yaml.Unmarshal([]byte(c), inputConfig); err != nil {
			return fmt.Errorf("unable to parse %s\nyaml: %s\n %v", fileName, string(c), err)
		}
		err = yaml.UnmarshalStrict([]byte(c), inputConfig)
		if err != nil {
			return fmt.Errorf("unable to UnmarshalStrict %s\nyaml: %s\n %v", inputConfig, string(c), err)
		}
		return nil
	}
	return fmt.Errorf("cluster spec file %s is invalid or does not contain kind %v", fileName, inputConfig)
}

func (Input *Input) NewBundleFromInput() (api.PackageBundleSpec, string, error) {
	packageBundleSpec := api.PackageBundleSpec{}
	if Input.Name == "" || Input.KubernetesVersion == "" {
		return packageBundleSpec, "", fmt.Errorf("error: Empty input field from `Name` or `KubernetesVersion`.")
	}
	var name string
	name, ok := os.LookupEnv("CODEBUILD_BUILD_NUMBER")
	if !ok {
		name = Input.Name
	} else {
		version := strings.Split(Input.KubernetesVersion, ".")
		name = fmt.Sprintf("v1-%s-%s", version[1], name)
	}
	for _, org := range Input.Packages {
		fmt.Printf("org=%v\n", org)
		for _, project := range org.Projects {
			bundlePkg, err := project.NewPackageFromInput()
			if err != nil {
				BundleLog.Error(err, "Unable to complete NewBundleFromInput from ecr lookup failure")
				return packageBundleSpec, "", err
			}
			packageBundleSpec.Packages = append(packageBundleSpec.Packages, *bundlePkg)
		}
	}
	return packageBundleSpec, name, nil
}
