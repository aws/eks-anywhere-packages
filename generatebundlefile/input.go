package main

import (
	"fmt"
	"io/ioutil"
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
	bundle := &api.PackageBundle{}
	err := ParseBundle(fileName, bundle)
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

func ParseBundle(fileName string, bundle *api.PackageBundle) error {
	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("unable to read file due to: %v", err)
	}
	for _, c := range strings.Split(string(content), YamlSeparator) {
		if err = yaml.Unmarshal([]byte(c), bundle); err != nil {
			return fmt.Errorf("unable to parse %s\nyaml: %s\n %v", fileName, string(c), err)
		}
		err = yaml.UnmarshalStrict([]byte(c), bundle)
		if err != nil {
			return fmt.Errorf("unable to UnmarshalStrict %v\nyaml: %s\n %v", bundle, string(c), err)
		}
		return nil
	}
	return fmt.Errorf("cluster spec file %s is invalid or does not contain kind %v", fileName, bundle)
}

func ParseInputConfig(fileName string, inputConfig *Input) error {
	content, err := ioutil.ReadFile(fileName)
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
	packageBundleSpec.KubeVersion = Input.KubernetesVersion
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
	return packageBundleSpec, Input.Name, nil
}
