package bundle

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/yaml"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/artifacts"
)

type Manager interface {
	// Update the bundle returns true if there are changes
	Update(ctx context.Context, newBundle *api.PackageBundle) (bool, error)

	// ProcessLatestBundle make sure we save the latest bundle
	ProcessLatestBundle(ctx context.Context, bundle *api.PackageBundle) error

	// LatestBundle pulls the bundle tagged with "latest" from the bundle source.
	LatestBundle(ctx context.Context, baseRef string) (
		*api.PackageBundle, error)

	// DownloadBundle downloads the bundle with a given tag.
	DownloadBundle(ctx context.Context, ref string) (
		*api.PackageBundle, error)

	// SortBundlesDescending sort bundles to latest first
	SortBundlesDescending(bundles []api.PackageBundle)
}

type bundleManager struct {
	log               logr.Logger
	kubeServerVersion discovery.ServerVersionInterface
	puller            artifacts.Puller
	bundleClient      Client
}

func NewBundleManager(log logr.Logger, serverVersion discovery.ServerVersionInterface,
	puller artifacts.Puller, bundleClient Client) (manager *bundleManager) {
	manager = &bundleManager{
		log:               log,
		kubeServerVersion: serverVersion,
		puller:            puller,
		bundleClient:      bundleClient,
	}

	return manager
}

var _ Manager = (*bundleManager)(nil)

func (m bundleManager) Update(ctx context.Context, newBundle *api.PackageBundle) (bool, error) {

	active, err := m.bundleClient.IsActive(ctx, newBundle)
	if err != nil {
		return false, err
	}

	kubeVersion, err := m.apiVersion()
	if err != nil {
		return false, fmt.Errorf("retrieving k8s API version: %w", err)
	}

	kubeMatches := newBundle.KubeVersionMatches(kubeVersion)

	if newBundle.Namespace != api.PackageNamespace || !kubeMatches {
		if newBundle.Status.State != api.PackageBundleStateIgnored {
			newBundle.Spec.DeepCopyInto(&newBundle.Status.Spec)
			newBundle.Status.State = api.PackageBundleStateIgnored
			return true, nil
		}
		return false, nil
	}
	if !active {
		if newBundle.Status.State == api.PackageBundleStateActive {
			m.log.V(6).Info("mark bundle inactive", "bundle", newBundle.Name)
			newBundle.Spec.DeepCopyInto(&newBundle.Status.Spec)
			newBundle.Status.State = api.PackageBundleStateInactive
			return true, nil
		}
		return false, nil
	}

	updateAvailable := false
	knownBundles := &api.PackageBundleList{}
	err = m.bundleClient.GetBundleList(ctx, knownBundles)
	if err != nil {
		return false, fmt.Errorf("getting bundle list: %s", err)
	}
	allBundles := knownBundles.Items
	if len(allBundles) > 0 {
		m.SortBundlesDescending(allBundles)
		if allBundles[0].Name != newBundle.Name {
			updateAvailable = true
		}
	}

	change := false
	if updateAvailable {
		if newBundle.Status.State != api.PackageBundleStateUpgradeAvailable {
			m.log.V(6).Info("mark update available", "bundle", newBundle.Name)
			newBundle.Status.State = api.PackageBundleStateUpgradeAvailable
			change = true
		}
	} else if newBundle.Status.State != api.PackageBundleStateActive {
		m.log.V(6).Info("mark active", "bundle", newBundle.Name)
		newBundle.Status.State = api.PackageBundleStateActive
		change = true
	}

	newBundle.Spec.DeepCopyInto(&newBundle.Status.Spec)
	return change, nil
}

// SortBundlesDescending will sort a slice of bundles in descending order so
// that the newest (greatest) bundle will be displayed first.
func (m bundleManager) SortBundlesDescending(bundles []api.PackageBundle) {
	sortFn := func(i, j int) bool {
		return bundles[j].LessThan(&bundles[i])
	}
	sort.Slice(bundles, sortFn)
}

// LatestBundle pulls the bundle tagged with "latest" from the bundle source.
//
// It returns an error if the bundle it retrieves is empty. This is because an
// empty file would be successfully parsed and a Zero-value PackageBundle
// returned, which is not acceptable.
func (m *bundleManager) LatestBundle(ctx context.Context, baseRef string) (
	*api.PackageBundle, error) {

	kubeVersion, err := m.apiVersion()
	if err != nil {
		return nil, fmt.Errorf("retrieving k8s API version: %w", err)
	}
	tag := "latest"
	ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

	return m.DownloadBundle(ctx, ref)
}

func (m *bundleManager) DownloadBundle(ctx context.Context, ref string) (*api.PackageBundle, error) {

	data, err := m.puller.Pull(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("pulling package bundle: %s", err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("package bundle artifact is empty")
	}

	bundle := &api.PackageBundle{}
	err = yaml.Unmarshal(data, bundle)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling package bundle: %s", err)
	}

	return bundle, nil
}

func (m *bundleManager) ProcessLatestBundle(ctx context.Context, latestBundle *api.PackageBundle) error {
	knownBundles := &api.PackageBundleList{}
	err := m.bundleClient.GetBundleList(ctx, knownBundles)
	if err != nil {
		return fmt.Errorf("getting bundle list: %s", err)
	}

	for _, b := range knownBundles.Items {
		if b.Name == latestBundle.Name {
			return nil
		}
	}

	err = m.bundleClient.CreateBundle(ctx, latestBundle)
	if err != nil {
		return fmt.Errorf("creating new package bundle: %s", err)
	}

	return nil
}

func kubeVersion(name string) (string, error) {
	matches := kubeVersionRe.FindStringSubmatch(name)
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("no kubernetes version found in %q", name)
}

var kubeVersionRe = regexp.MustCompile(`^(v[^-]+)-.*$`)

func (m *bundleManager) apiVersion() (string, error) {
	info, err := m.kubeServerVersion.ServerVersion()
	if err != nil {
		return "", fmt.Errorf("getting server version: %s", err)
	}
	version := fmt.Sprintf("v%s-%s", info.Major, info.Minor)
	// The minor version can have a trailing + character that we don't want.
	version = strings.ReplaceAll(version, "+", "")

	return version, nil
}
