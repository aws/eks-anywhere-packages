package bundle

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/version"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

type Manager interface {
	// ProcessBundle returns true if there are changes
	ProcessBundle(ctx context.Context, newBundle *api.PackageBundle) (bool, error)

	// ProcessBundleController process the bundle controller
	ProcessBundleController(ctx context.Context, pbc *api.PackageBundleController) error

	// SortBundlesDescending sort bundles latest first
	SortBundlesDescending(bundles []api.PackageBundle)
}

type bundleManager struct {
	log            logr.Logger
	bundleClient   Client
	registryClient RegistryClient
	info           version.Info
}

func NewBundleManager(log logr.Logger, info version.Info, registryClient RegistryClient, bundleClient Client) *bundleManager {
	return &bundleManager{
		log:            log,
		bundleClient:   bundleClient,
		registryClient: registryClient,
		info:           info,
	}
}

var _ Manager = (*bundleManager)(nil)

func (m bundleManager) ProcessBundle(ctx context.Context, newBundle *api.PackageBundle) (bool, error) {
	if newBundle.Namespace != api.PackageNamespace {
		if newBundle.Status.State != api.PackageBundleStateIgnored {
			newBundle.Spec.DeepCopyInto(&newBundle.Status.Spec)
			newBundle.Status.State = api.PackageBundleStateIgnored
			m.log.V(6).Info("update", "bundle", newBundle.Name, "state", newBundle.Status.State)
			return true, nil
		}
		return false, nil
	}

	if !newBundle.IsValidVersion() {
		if newBundle.Status.State != api.PackageBundleStateInvalid {
			newBundle.Spec.DeepCopyInto(&newBundle.Status.Spec)
			newBundle.Status.State = api.PackageBundleStateInvalid
			m.log.V(6).Info("update", "bundle", newBundle.Name, "state", newBundle.Status.State)
			return true, nil
		}
		return false, nil
	}

	if newBundle.Status.State != api.PackageBundleStateAvailable {
		newBundle.Spec.DeepCopyInto(&newBundle.Status.Spec)
		newBundle.Status.State = api.PackageBundleStateAvailable
		m.log.V(6).Info("update", "bundle", newBundle.Name, "state", newBundle.Status.State)
		return true, nil
	}
	return false, nil
}

// SortBundlesDescending will sort a slice of bundles in descending order so
// that the newest (greatest) bundle will be displayed first.
func (m bundleManager) SortBundlesDescending(bundles []api.PackageBundle) {
	sortFn := func(i, j int) bool {
		return bundles[j].LessThan(&bundles[i])
	}
	sort.Slice(bundles, sortFn)
}

func (m *bundleManager) ProcessBundleController(ctx context.Context, pbc *api.PackageBundleController) error {
	kubeVersion := FormatKubeServerVersion(m.info)
	latestBundle, err := m.registryClient.LatestBundle(ctx, pbc.Spec.Source.GetRef(), kubeVersion)
	if err != nil {
		m.log.Error(err, "Unable to get latest bundle")
		if pbc.Status.State == api.BundleControllerStateActive {
			pbc.Status.State = api.BundleControllerStateDisconnected
			err = m.bundleClient.SaveStatus(ctx, pbc)
			if err != nil {
				return fmt.Errorf("updating %s status to %s: %s", pbc.Name, pbc.Status.State, err)
			}
		}
		return nil
	}

	knownBundles := &api.PackageBundleList{}
	err = m.bundleClient.GetBundleList(ctx, knownBundles)
	if err != nil {
		return fmt.Errorf("getting bundle list: %s", err)
	}
	sortedBundles := knownBundles.Items
	m.SortBundlesDescending(sortedBundles)

	latestBundleIsKnown := false
	latestBundleIsCurrentBundle := true
	for _, b := range sortedBundles {
		if b.Name == latestBundle.Name {
			latestBundleIsKnown = true
			break
		}
		latestBundleIsCurrentBundle = false
	}

	if !latestBundleIsKnown {
		err = m.bundleClient.CreateBundle(ctx, latestBundle)
		if err != nil {
			return fmt.Errorf("creating new package bundle: %s", err)
		}
	}

	switch pbc.Status.State {
	case api.BundleControllerStateActive:
		if latestBundleIsCurrentBundle {
			break
		}
		pbc.Status.State = api.BundleControllerStateUpgradeAvailable
		m.log.V(6).Info("update", "PackageBundleController", pbc.Name, "state", pbc.Status.State)
		err = m.bundleClient.SaveStatus(ctx, pbc)
		if err != nil {
			return fmt.Errorf("updating %s status to %s: %s", pbc.Name, pbc.Status.State, err)
		}
	case api.BundleControllerStateUpgradeAvailable:
		if !latestBundleIsCurrentBundle {
			break
		}
		pbc.Status.State = api.BundleControllerStateActive
		m.log.V(6).Info("update", "PackageBundleController", pbc.Name, "state", pbc.Status.State)
		err = m.bundleClient.SaveStatus(ctx, pbc)
		if err != nil {
			return fmt.Errorf("updating %s status to %s: %s", pbc.Name, pbc.Status.State, err)
		}
	case api.BundleControllerStateDisconnected:
		pbc.Status.State = api.BundleControllerStateActive
		m.log.V(6).Info("update", "PackageBundleController", pbc.Name, "state", pbc.Status.State)
		err = m.bundleClient.SaveStatus(ctx, pbc)
		if err != nil {
			return fmt.Errorf("updating %s status to %s: %s", pbc.Name, pbc.Status.State, err)
		}
	default:
		if pbc.Spec.ActiveBundle != "" {
			pbc.Status.State = api.BundleControllerStateActive
			m.log.V(6).Info("update", "PackageBundleController", pbc.Name, "state", pbc.Status.State)
			err = m.bundleClient.SaveStatus(ctx, pbc)
			if err != nil {
				return fmt.Errorf("updating %s status to %s: %s", pbc.Name, pbc.Status.State, err)
			}
		} else {
			pbc.Spec.ActiveBundle = latestBundle.Name
			m.log.V(6).Info("update", "PackageBundleController", pbc.Name, "activeBundle", pbc.Spec.ActiveBundle)
			err = m.bundleClient.Save(ctx, pbc)
			if err != nil {
				return fmt.Errorf("updating %s activeBundle to %s: %s", pbc.Name, pbc.Spec.ActiveBundle, err)
			}
		}
	}

	return nil
}

// FormatKubeServerVersion builds a string representation of the kubernetes
// server version.
func FormatKubeServerVersion(info version.Info) string {
	version := fmt.Sprintf("v%s-%s", info.Major, info.Minor)
	// The minor version can have a trailing + character that we don't want.
	return strings.ReplaceAll(version, "+", "")
}
