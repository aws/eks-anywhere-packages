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
	Update(newBundle *api.PackageBundle, isActive bool,
		allBundles []api.PackageBundle) bool

	// IsBundleKnown returns true if the bundle is in the list of known
	// bundles.
	IsBundleKnown(ctx context.Context,
		knownBundles []api.PackageBundle, bundle *api.PackageBundle) bool

	// LatestBundle pulls the bundle tagged with "latest" from the bundle source.
	LatestBundle(ctx context.Context, baseRef string) (
		*api.PackageBundle, error)

	// DownloadBundle downloads the bundle with a given tag.
	DownloadBundle(ctx context.Context, ref string) (
		*api.PackageBundle, error)

	SortBundlesNewestFirst(bundles []api.PackageBundle)
}

type bundleManager struct {
	log               logr.Logger
	kubeServerVersion discovery.ServerVersionInterface
	puller            artifacts.Puller
}

func NewBundleManager(log logr.Logger, serverVersion discovery.ServerVersionInterface,
	puller artifacts.Puller) (manager *bundleManager) {
	manager = &bundleManager{
		log:               log,
		kubeServerVersion: serverVersion,
		puller:            puller,
	}

	// This is temporary
	var newBundle api.PackageBundle
	_ = manager.Update(&newBundle, false, nil)

	return manager
}

var _ Manager = (*bundleManager)(nil)

func (m bundleManager) Update(newBundle *api.PackageBundle, active bool,
	allBundles []api.PackageBundle) bool {

	if !active {
		if newBundle.Status.State == api.PackageBundleStateActive {
			newBundle.Status.State = api.PackageBundleStateInactive
			return true
		}
		return false
	}
	if newBundle.Status.State != api.PackageBundleStateActive {
		newBundle.Status.State = api.PackageBundleStateActive
	}

	// allBundles should never be nil or empty in production, but for testing
	// it's much easier to handle a nil case.
	if active && allBundles != nil && len(allBundles) > 0 {
		m.SortBundlesNewestFirst(allBundles)
		if allBundles[0].Name != newBundle.Name {
			newBundle.Status.State = api.PackageBundleStateUpgradeAvailable
		}
	}

	newBundle.Spec.DeepCopyInto(&newBundle.Status.Spec)
	return true
}

// SortBundlesNewestFirst will sort a slice of bundles so that the newest is first.
func (m bundleManager) SortBundlesNewestFirst(bundles []api.PackageBundle) {
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

	// TODO Can the data be validated here? Is that useful?

	return bundle, nil
}

func (m *bundleManager) IsBundleKnown(ctx context.Context,
	knownBundles []api.PackageBundle,
	bundle *api.PackageBundle) bool {

	for _, b := range knownBundles {
		if b.Name == bundle.Name {
			return true
		}
	}

	return false
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
