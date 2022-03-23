package driver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/storage/driver"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

// helmDriver implements PackageDriver to install packages from Helm charts.
type helmDriver struct {
	cfg      *action.Configuration
	log      logr.Logger
	settings *cli.EnvSettings
}

var _ PackageDriver = (*helmDriver)(nil)

func NewHelm(log logr.Logger) (*helmDriver, error) {
	settings := cli.New()
	//TODO: Allow configuring the client's credentials from a secret
	//see: https://github.com/aws/eks-anywhere-packages/issues/20
	client, err := registry.NewClient()
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

func (d *helmDriver) Install(ctx context.Context,
	name string, namespace string, source api.PackageOCISource, values map[string]interface{}) error {
	var err error

	install := action.NewInstall(d.cfg)
	fullName := d.prefixName(name)

	install.Version = source.Version
	install.ReleaseName = fullName
	install.Namespace = namespace

	helmChart, err := d.getChart(install, source)
	if err != nil {
		return fmt.Errorf("loading helm chart %s: %w", fullName, err)
	}

	// Check if there exists a matching helm release.
	get := action.NewGet(d.cfg)
	_, err = get.Run(fullName)
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			return d.createRelease(ctx, install, helmChart, values)
		}
		return fmt.Errorf("getting helm release %s: %w", fullName, err)
	}

	err = d.upgradeRelease(ctx, fullName, helmChart, values)
	if err != nil {
		return fmt.Errorf("upgrading helm chart %s: %w", name, err)
	}

	return nil
}

func (d *helmDriver) getChart(install *action.Install, source api.PackageOCISource) (*chart.Chart, error) {
	url := getChartURL(source)
	chartPath, err := install.LocateChart(url, d.settings)
	if err != nil {
		return nil, fmt.Errorf("locating helm chart %s tag %s: %w", url, source.Digest, err)
	}
	return loader.Load(chartPath)
}

func getChartURL(source api.PackageOCISource) string {
	return "oci://" + source.AsRepoURI()
}

func (d *helmDriver) createRelease(ctx context.Context,
	install *action.Install, helmChart *chart.Chart, values map[string]interface{}) error {
	_, err := install.RunWithContext(ctx, helmChart, values)
	if err != nil {
		return fmt.Errorf("installing helm chart %s: %w", install.ReleaseName, err)
	}

	return nil
}

// helmChartURLIsPrefixed detects if the given URL has an acceptable scheme
// prefix.
func helmChartURLIsPrefixed(url string) bool {
	return strings.HasPrefix(url, "https://") ||
		strings.HasPrefix(url, "http://") ||
		strings.HasPrefix(url, "oci://")
}

// upgradeRelease instructs helm to upgrade a release.
func (d *helmDriver) upgradeRelease(ctx context.Context, name string,
	helmChart *chart.Chart, values map[string]interface{}) (err error) {

	// TODO Increased efficiency might be achieved by avoiding running helm
	// upgrade unless changes in the values are detected. For POC, run helm
	// every time and rely on its idempotency.
	upgrade := action.NewUpgrade(d.cfg)
	_, err = upgrade.RunWithContext(ctx, name, helmChart, values)
	if err != nil {
		return fmt.Errorf("upgrading helm release %s: %w", name, err)
	}

	return nil
}

func (d *helmDriver) Uninstall(_ context.Context, name string) (err error) {
	uninstall := action.NewUninstall(d.cfg)
	_, err = uninstall.Run(d.prefixName(name))
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			return nil
		}
		return fmt.Errorf("uninstalling helm chart %s: %w", name, err)
	}

	return nil
}

// helmLog wraps logr.Logger to make it compatible with helm's DebugLog.
func helmLog(log logr.Logger) action.DebugLog {
	return func(template string, args ...interface{}) {
		log.Info(fmt.Sprintf(template, args...))
	}
}

// helmPrefix is a prefix prepending to EKS Anywhere Package helm release names.
const helmPrefix = "eks-anywhere-test"

// helmReleaseName is a helper function for building a prefixed release name.
func (d *helmDriver) prefixName(name string) string {
	return fmt.Sprintf("%s-%s", helmPrefix, name)
}
