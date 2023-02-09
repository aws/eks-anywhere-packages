package driver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/storage/driver"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	auth "github.com/aws/eks-anywhere-packages/pkg/authenticator"
	packagesRegistry "github.com/aws/eks-anywhere-packages/pkg/registry"
)

const (
	varHelmUpgradeMaxHistory = 2
)

// helmDriver implements PackageDriver to install packages from Helm charts.
type helmDriver struct {
	cfg        *action.Configuration
	secretAuth auth.Authenticator
	tcc        auth.TargetClusterClient
	log        logr.Logger
	settings   *cli.EnvSettings
}

var _ PackageDriver = (*helmDriver)(nil)

func NewHelm(log logr.Logger, secretAuth auth.Authenticator, tcc auth.TargetClusterClient) *helmDriver {
	return &helmDriver{
		secretAuth: secretAuth,
		tcc:        tcc,
		log:        log,
	}
}

func (d *helmDriver) Initialize(ctx context.Context, clusterName string) (err error) {
	err = d.secretAuth.Initialize(clusterName)
	if err != nil {
		d.log.Info("failed to change target cluster for secrets")
	}
	err = d.tcc.Initialize(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("initialiing target cluster %s client for helm driver: %w", clusterName, err)
	}

	d.settings = cli.New()

	insecure := packagesRegistry.GetRegistryInsecure(clusterName)
	caFile := packagesRegistry.GetClusterCertificateFileName(clusterName)
	client, err := newRegistryClient("", "", caFile, insecure, d.settings)
	if err != nil {
		return fmt.Errorf("creating registry client for helm driver: %w", err)
	}

	d.cfg = &action.Configuration{RegistryClient: client}
	err = d.cfg.Init(d.tcc, d.settings.Namespace(), os.Getenv("HELM_DRIVER"), helmLog(d.log))
	if err != nil {
		return fmt.Errorf("initializing helm driver: %w", err)
	}

	return nil
}

func (d *helmDriver) Install(ctx context.Context,
	name string, namespace string, createNamespace bool, source api.PackageOCISource, values map[string]interface{},
) error {
	var err error
	install := action.NewInstall(d.cfg)
	install.Version = source.Version
	install.ReleaseName = name
	install.CreateNamespace = createNamespace

	helmChart, err := d.getChart(install, source)
	if err != nil {
		return fmt.Errorf("loading helm chart %s: %w", name, err)
	}
	// If no target namespace provided read chart values to find namespace
	if namespace == "" {
		if chartNS, ok := helmChart.Values["defaultNamespace"]; ok {
			namespace = chartNS.(string)
		} else {
			// Fall back case of assuming its default
			namespace = "default"
		}
	}
	install.Namespace = namespace

	// Update values with imagePullSecrets
	// If no secret values we should still continue as it could be case of public registry or local registry
	secretvals, err := d.secretAuth.GetSecretValues(ctx, namespace)
	if err != nil {
		secretvals = nil
		// Continue as its possible that a private registry is being used here and thus no data necessary
	}
	for key, val := range secretvals {
		values[key] = val
	}

	// Check if there exists a matching helm release.
	get := action.NewGet(d.cfg)
	_, err = get.Run(name)
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			// Trigger configmap updates and namespace before trying to install charts
			if err := d.secretAuth.AddToConfigMap(ctx, name, namespace); err != nil {
				d.log.Info("failed to Update ConfigMap with installed namespace", "error", err)
			}
			if err := d.secretAuth.AddSecretToAllNamespace(ctx); err != nil {
				d.log.Info("failed to Update Secret in all namespaces", "error", err)
			}
			err = d.createRelease(ctx, install, helmChart, values)
			if err != nil {
				err1 := d.secretAuth.DelFromConfigMap(ctx, name, namespace)
				if err1 != nil {
					d.log.Info("failed to remove namespace from configmap")
				}
				return err
			}
			// Failsafe in event namespace is created via the charts
			if err := d.secretAuth.AddSecretToAllNamespace(ctx); err != nil {
				d.log.Info("failed to Update Secret in all namespaces", "error", err)
			}
			return nil
		}
		return fmt.Errorf("getting helm release %s: %w", name, err)
	}

	err = d.upgradeRelease(ctx, name, helmChart, values)
	if err != nil {
		return fmt.Errorf("upgrading helm chart %s: %w", name, err)
	}

	// Update installed-namespaces on successful install
	err = d.secretAuth.AddToConfigMap(ctx, name, namespace)
	if err != nil {
		d.log.Info("failed to Update ConfigMap with installed namespace", "error", err)
	}
	if err := d.secretAuth.AddSecretToAllNamespace(ctx); err != nil {
		d.log.Info("failed to Update Secret in all namespaces", "error", err)
	}

	err = d.upgradeRelease(ctx, name, helmChart, values)
	if err != nil {
		return fmt.Errorf("upgrading helm chart %s: %w", name, err)
	}

	if err := d.secretAuth.AddSecretToAllNamespace(ctx); err != nil {
		d.log.Info("failed to Update Secret in all namespaces", "error", err)
	}

	return nil
}

func (d *helmDriver) getChart(install *action.Install, source api.PackageOCISource) (*chart.Chart, error) {
	url := source.GetChartUri()
	chartPath, err := install.LocateChart(url, d.settings)
	if err != nil {
		return nil, fmt.Errorf("locating helm chart %s tag %s: %w", url, source.Digest, err)
	}
	return loader.Load(chartPath)
}

func (d *helmDriver) createRelease(ctx context.Context,
	install *action.Install, helmChart *chart.Chart, values map[string]interface{},
) error {
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
	helmChart *chart.Chart, values map[string]interface{},
) (err error) {
	// upgrade unless changes in the values are detected. For POC, run helm
	// every time and rely on its idempotency.
	upgrade := action.NewUpgrade(d.cfg)
	// Limit history saved as secret for resource limit
	upgrade.MaxHistory = varHelmUpgradeMaxHistory
	_, err = upgrade.RunWithContext(ctx, name, helmChart, values)
	if err != nil {
		return fmt.Errorf("upgrading helm release %s: %w", name, err)
	}

	return nil
}

func (d *helmDriver) Uninstall(ctx context.Context, name string) (err error) {
	uninstall := action.NewUninstall(d.cfg)
	rel, err := uninstall.Run(name)
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			return nil
		}
		return fmt.Errorf("uninstalling helm chart %s: %w", name, err)
	}
	err = d.secretAuth.DelFromConfigMap(ctx, name, rel.Release.Namespace)
	if err != nil {
		d.log.Info("failed to remove namespace from configmap")
	}
	return nil
}

// helmLog wraps logr.Logger to make it compatible with helm's DebugLog.
func helmLog(log logr.Logger) action.DebugLog {
	return func(template string, args ...interface{}) {
		log.Info(fmt.Sprintf(template, args...))
	}
}

func (d *helmDriver) IsConfigChanged(_ context.Context, name string, values map[string]interface{}) (bool, error) {
	get := action.NewGet(d.cfg)
	rel, err := get.Run(name)
	if err != nil {
		d.log.Info("Installation not found %q: %w", name, err)
		return true, nil
	}

	// Check imagePullSecret not defined in config
	if _, exist := values["imagePullSecrets"]; !exist {
		// Check if imagePullSecrets was added by driver
		if val, ok := rel.Config["imagePullSecrets"]; ok {
			values["imagePullSecrets"] = val
		}
	}

	return !reflect.DeepEqual(values, rel.Config), nil
}

func newRegistryClient(certFile, keyFile, caFile string, insecureSkipTLSverify bool, settings *cli.EnvSettings) (*registry.Client, error) {
	if certFile != "" && keyFile != "" || caFile != "" || !insecureSkipTLSverify {
		registryClient, err := newRegistryClientWithTLS(certFile, keyFile, caFile, insecureSkipTLSverify, settings)
		if err != nil {
			return nil, err
		}
		return registryClient, nil
	}
	registryClient, err := newDefaultRegistryClient(settings)
	if err != nil {
		return nil, err
	}
	return registryClient, nil
}

func newDefaultRegistryClient(settings *cli.EnvSettings) (*registry.Client, error) {
	// Create a new registry client
	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptEnableCache(false),
		registry.ClientOptWriter(os.Stderr),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
	)
	if err != nil {
		return nil, err
	}
	return registryClient, nil
}

func newRegistryClientWithTLS(certFile, keyFile, caFile string, insecureSkipTLSverify bool, settings *cli.EnvSettings) (*registry.Client, error) {
	// Create a new registry client
	registryClient, err := registry.NewRegistryClientWithTLS(os.Stderr, certFile, keyFile, caFile, insecureSkipTLSverify,
		settings.RegistryConfig, settings.Debug,
	)
	if err != nil {
		return nil, err
	}
	return registryClient, nil
}
