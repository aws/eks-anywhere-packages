package bundle

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/artifacts"
)

const (
	ActiveBundleNamespace       = "eksa-packages"
	PackageBundleControllerName = "eksa-packages-bundle-controller"
)

type Manager interface {
	// IsActive returns true if the given namespace and name matche the active
	// bundle's.
	IsActive(ctx context.Context, client client.Client,
		namespacedName types.NamespacedName) (bool, error)

	// ActiveBundle retrieves the currently active bundle.
	ActiveBundle(ctx context.Context, client client.Client) (
		*api.PackageBundle, error)

	// Update the bundle returns true if there are changes
	Update(newBundle *api.PackageBundle, isActive bool,
		allBundles []api.PackageBundle) bool

	// IsBundleKnown returns true if the bundle is in the list of known
	// bundles.
	IsBundleKnown(ctx context.Context,
		knownBundles []api.PackageBundle, bundle *api.PackageBundle) bool

	// IsBundleOlderThan returns true if the current name is older than the
	// candidate.
	IsBundleOlderThan(current, candidate string) (bool, error)

	// LatestBundle pulls the bundle tagged with "latest" from the bundle source.
	LatestBundle(ctx context.Context, baseRef string) (
		*api.PackageBundle, error)

	// IsActiveController returns true if the given namespace and name matches
	// the active bundle controller's.
	IsActiveController(namespacedName types.NamespacedName) bool

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

func (m bundleManager) IsActive(ctx context.Context,
	client client.Client, key types.NamespacedName) (bool, error) {

	abc, err := m.getPackageBundleController(ctx, client)
	if err != nil {
		return false, err
	}

	return key.Namespace == ActiveBundleNamespace && key.Name == abc.Spec.ActiveBundle, nil
}

func (m bundleManager) getPackageBundleController(ctx context.Context,
	client client.Client) (*api.PackageBundleController, error) {
	abc := &api.PackageBundleController{}
	key := types.NamespacedName{
		Namespace: ActiveBundleNamespace,
		Name:      PackageBundleControllerName,
	}
	err := client.Get(ctx, key, abc)
	if err != nil {
		return nil, fmt.Errorf("getting PackageBundleController: %s", err)
	}

	return abc, nil
}

func (m bundleManager) Update(newBundle *api.PackageBundle, active bool,
	allBundles []api.PackageBundle) bool {

	if active && newBundle.Status.State != api.PackageBundleStateActive {
		newBundle.Status.State = api.PackageBundleStateActive
	} else if !active && newBundle.Status.State != api.PackageBundleStateInactive {
		newBundle.Status.State = api.PackageBundleStateInactive
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
		older, err := m.IsBundleOlderThan(bundles[i].Name, bundles[j].Name)
		if err != nil {
			return true
		}
		return !older
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

	data, err := m.puller.Pull(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("pulling package bundle: %s", err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("latest package bundle artifact is empty")
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

func (m *bundleManager) IsBundleOlderThan(current, candidate string) (bool, error) {
	if current == "" {
		return true, nil
	}

	curK8sVer, err := kubeVersion(current)
	if err != nil {
		return false, fmt.Errorf("parsing current kube version: %s", err)
	}

	newK8sVer, err := kubeVersion(candidate)
	if err != nil {
		return false, fmt.Errorf("parsing candidate kube version: %s", err)
	}

	if curK8sVer < newK8sVer {
		return true, nil
	}

	curBuildNum, err := buildNumber(current)
	if err != nil {
		return false, fmt.Errorf("parsing current build number: %s", err)
	}

	newBuildNum, err := buildNumber(candidate)
	if err != nil {
		return false, fmt.Errorf("parsing candidate build number: %s", err)
	}

	return curBuildNum < newBuildNum, nil
}

func kubeVersion(name string) (string, error) {
	matches := kubeVersionRe.FindStringSubmatch(name)
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("no kubernetes version found in %q", name)
}

var kubeVersionRe = regexp.MustCompile(`^(v[^-]+)-.*$`)

func buildNumber(name string) (int, error) {
	matches := bundleNameRe.FindStringSubmatch(name)
	if len(matches) > 1 {
		buildNumber, err := strconv.Atoi(matches[1])
		if err != nil {
			return 0, fmt.Errorf("parsing build number: %s", err)
		}

		return buildNumber, nil
	}

	return 0, fmt.Errorf("no build number found in %q", name)
}

var bundleNameRe = regexp.MustCompile(`^.*-(\d+)$`)

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

// ActiveBundle retrieves the bundle from which package are installed.
//
// It retrieves the name of the active bundle from the PackageBundleController,
// then uses the K8s API to retrieve and return the CRD for the active bundle
// itself.
func (m bundleManager) ActiveBundle(ctx context.Context, client client.Client) (*api.PackageBundle, error) {
	abc, err := m.getPackageBundleController(ctx, client)
	if err != nil {
		return nil, err
	}

	nn := types.NamespacedName{
		Namespace: ActiveBundleNamespace,
		Name:      abc.Spec.ActiveBundle,
	}
	bundle := &api.PackageBundle{}
	err = client.Get(ctx, nn, bundle)
	if err != nil {
		return nil, err
	}

	return bundle, nil
}

func (m bundleManager) IsActiveController(namespacedName types.NamespacedName) bool {
	return namespacedName.Name == PackageBundleControllerName &&
		namespacedName.Namespace == ActiveBundleNamespace
}
