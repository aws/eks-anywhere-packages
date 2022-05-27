package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
	"sigs.k8s.io/yaml"
)

// helmDriver implements PackageDriver to install packages from Helm charts.
type helmDriver struct {
	cfg      *action.Configuration
	log      logr.Logger
	settings *cli.EnvSettings
}

func NewHelm(log logr.Logger, authfile string) (*helmDriver, error) {
	settings := cli.New()
	options := registry.ClientOptCredentialsFile(authfile)
	client, err := registry.NewClient(options)
	if err != nil {
		return nil, fmt.Errorf("creating registry client while initializing helm driver: %w", err)
	}
	cfg := &action.Configuration{RegistryClient: client}
	err = cfg.Init(settings.RESTClientGetter(), settings.Namespace(),
		os.Getenv("HELM_DRIVER"), helmLog(log))
	if err != nil {
		return nil, fmt.Errorf("initializing helm driver: %w", err)
	}
	return &helmDriver{
		cfg:      cfg,
		log:      log,
		settings: settings,
	}, nil
}

// PullHelmChart will take in a a remote Helm URI and attempt to pull down the chart if it exists.
func (d *helmDriver) PullHelmChart(name, version string) (string, error) {
	if name == "" || version == "" {
		return "", fmt.Errorf("empty input for PullHelmChart, check flags")
	}
	install := action.NewInstall(d.cfg)
	install.ChartPathOptions.Version = version
	if !strings.HasPrefix(name, "oci://") {
		name = fmt.Sprintf("oci://%s", name)
	}
	chartPath, err := install.LocateChart(name, d.settings)
	if err != nil || chartPath == "" {
		return "", fmt.Errorf("running the Helm LocateChart command, you might need run an AWS ECR Login: %w", err)
	}
	return chartPath, nil
}

// helmLog wraps logr.Logger to make it compatible with helm's DebugLog.
func helmLog(log logr.Logger) action.DebugLog {
	return func(template string, args ...interface{}) {
		log.Info(fmt.Sprintf(template, args...))
	}
}

// UnTarHelmChart will attempt to move the helm chart out of the helm cache, by untaring it to the pwd and creating the filesystem to unpack it into.
func UnTarHelmChart(chartRef, chartPath, dest string) error {
	if chartRef == "" || chartPath == "" || dest == "" {
		return fmt.Errorf("Empty input value given for UnTarHelmChart")
	}
	_, err := os.Stat(dest)
	if err != nil {
		return fmt.Errorf("Not valid File %w", err)
	}
	if os.IsNotExist(err) {
		if _, err := os.Stat(chartPath); err != nil {
			if err := os.MkdirAll(chartPath, 0755); err != nil {
				return errors.Wrap(err, "failed to untar (mkdir)")
			}
		} else {
			return errors.Errorf("failed to untar: a file or directory with the name %s already exists", dest)
		}
	}
	// Untar the files, and create the directory structure
	return chartutil.ExpandFile(dest, chartRef)
}

// hasRequires checks for the existance of the requires.yaml within the helm directory
func hasRequires(helmdir string) (string, error) {
	requires := filepath.Join(helmdir, "requires.yaml")
	info, err := os.Stat(requires)
	if os.IsNotExist(err) {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("Found Dir, not requires.yaml file")
	}
	return requires, nil
}

// validateHelmRequires runs the parse file into struct function, and validations
func validateHelmRequires(fileName string) (*Requires, error) {
	helmrequires := &Requires{}
	err := parseHelmRequires(fileName, helmrequires)
	if err != nil {
		return nil, err
	}
	err = validateHelmRequiresContent(helmrequires)
	if err != nil {
		return nil, err
	}
	return helmrequires, err
}

// validateHelmRequiresContent loops over the validation tests
func validateHelmRequiresContent(helmrequires *Requires) error {
	for _, v := range helmRequiresValidations {
		if err := v(helmrequires); err != nil {
			return err
		}
	}
	return nil
}

var helmRequiresValidations = []func(*Requires) error{
	validateHelmRequiresName,
}

func validateHelmRequiresName(helmrequires *Requires) error {
	err := helmrequires.validateHelmRequiresNotEmpty()
	if err != nil {
		return err
	}
	return nil
}

// validateHelmRequiresNotEmpty checks that it has at least one image in the spec
func (helmrequires *Requires) validateHelmRequiresNotEmpty() error {
	// Check if Projects are listed
	if len(helmrequires.Spec.Images) < 1 {
		return fmt.Errorf("Should use non-empty list of images for requires")
	}
	return nil
}

// parseHelmRequires will attempt to unpack the requires.yaml into the Go struct `Requires`
func parseHelmRequires(fileName string, helmrequires *Requires) error {
	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("unable to read file due to: %v", err)
	}
	for _, c := range strings.Split(string(content), YamlSeparator) {
		if err = yaml.Unmarshal([]byte(c), helmrequires); err != nil {
			return fmt.Errorf("unable to parse %s\nyaml: %s\n %v", fileName, string(c), err)
		}
		err = yaml.UnmarshalStrict([]byte(c), helmrequires)
		if err != nil {
			return fmt.Errorf("unable to UnmarshalStrict %s\nyaml: %s\n %v", helmrequires, string(c), err)
		}
		return nil
	}
	return fmt.Errorf("cluster spec file %s is invalid or does not contain kind %v", fileName, helmrequires)
}
