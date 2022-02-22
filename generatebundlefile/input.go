package main

import (
	"fmt"
	"io/ioutil"
	"strings"

	"sigs.k8s.io/yaml"

	api "github.com/aws/modelrocket-add-ons/api/v1alpha1"
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

func (Input *Input) NewBundleFromInput() (api.PackageBundleSpec, error) {
	packageBundleSpec := api.PackageBundleSpec{}
	packageBundleSpec.KubeVersion = Input.KubernetesVersion
	for _, org := range Input.Packages {
		for _, project := range org.Projects {
			bundlePkg, err := project.NewPackageFromInput()
			if err != nil {
				BundleLog.Error(err, "Unable to complete NewBundleFromInput from ecr lookup failure")
				return packageBundleSpec, err
			}
			packageBundleSpec.Packages = append(packageBundleSpec.Packages, *bundlePkg)
		}
	}
	return packageBundleSpec, nil
}
