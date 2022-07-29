package bundle

import (
	"context"
	"fmt"
	"sort"

	"github.com/go-logr/logr"

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
	log               logr.Logger
	bundleClient      Client
	registryClient    RegistryClient
	kubeVersionClient KubeVersionClient
}

func NewBundleManager(log logr.Logger, kubeVersionClient KubeVersionClient, registryClient RegistryClient, bundleClient Client) *bundleManager {
	return &bundleManager{
		log:               log,
		bundleClient:      bundleClient,
		registryClient:    registryClient,
		kubeVersionClient: kubeVersionClient,
	}
}

var _ Manager = (*bundleManager)(nil)

func (m bundleManager) ProcessBundle(ctx context.Context, newBundle *api.PackageBundle) (bool, error) {

	active, err := m.bundleClient.IsActive(ctx, newBundle)
	if err != nil {
		return false, fmt.Errorf("getting active bundle: %s", err)
	}

	if newBundle.Namespace != api.PackageNamespace {
		if newBundle.Status.State != api.PackageBundleStateIgnored {
			newBundle.Spec.DeepCopyInto(&newBundle.Status.Spec)
			newBundle.Status.State = api.PackageBundleStateIgnored
			return true, nil
		}
		return false, nil
	}

	kubeVersion, err := m.kubeVersionClient.ApiVersion()
	if err != nil {
		return false, fmt.Errorf("getting kube version: %w", err)
	}

	matches, err := newBundle.KubeVersionMatches(kubeVersion)
	if !matches {
		if err != nil {
			if newBundle.Status.State != api.PackageBundleStateInvalidVersion {
				newBundle.Spec.DeepCopyInto(&newBundle.Status.Spec)
				newBundle.Status.State = api.PackageBundleStateInvalidVersion
				return true, nil
			}
		} else if newBundle.Status.State != api.PackageBundleStateIgnoredVersion {
			newBundle.Spec.DeepCopyInto(&newBundle.Status.Spec)
			newBundle.Status.State = api.PackageBundleStateIgnoredVersion
			return true, nil
		}
		return false, nil
	}

	if !active {
		if newBundle.Status.State == api.PackageBundleStateActive {
			newBundle.Spec.DeepCopyInto(&newBundle.Status.Spec)
			newBundle.Status.State = api.PackageBundleStateInactive
			m.log.V(6).Info("update", "bundle", newBundle.Name, "state", newBundle.Status.State)
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
			newBundle.Status.State = api.PackageBundleStateUpgradeAvailable
			m.log.V(6).Info("update", "bundle", newBundle.Name, "state", newBundle.Status.State)
			change = true
		}
	} else if newBundle.Status.State != api.PackageBundleStateActive {
		newBundle.Status.State = api.PackageBundleStateActive
		m.log.V(6).Info("update", "bundle", newBundle.Name, "state", newBundle.Status.State)
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

func (m *bundleManager) ProcessBundleController(ctx context.Context, pbc *api.PackageBundleController) error {
	kubeVersion, err := m.kubeVersionClient.ApiVersion()
	if err != nil {
		return fmt.Errorf("getting kube version: %s", err)
	}

	latestBundle, err := m.registryClient.LatestBundle(ctx, pbc.Spec.Source.BaseRef(), kubeVersion)
	if err != nil {
		m.log.Error(err, "Unable to get latest bundle")
		if pbc.Status.State == api.BundleControllerStateActive {
			m.log.Error(err, "marking disconnected")
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

	found := false
	latest := true
	for _, b := range knownBundles.Items {
		if b.Name == latestBundle.Name {
			found = true
			break
		}
		latest = false
	}

	if !found {
		err = m.bundleClient.CreateBundle(ctx, latestBundle)
		if err != nil {
			return fmt.Errorf("creating new package bundle: %s", err)
		}
	}

	switch pbc.Status.State {
	case api.BundleControllerStateActive:
		if latest {
			break
		}
		pbc.Status.State = api.BundleControllerStateUpgradeAvailable
		m.log.V(6).Info("update", "PackageBundleController", pbc.Name, "state", pbc.Status.State)
		err = m.bundleClient.SaveStatus(ctx, pbc)
		if err != nil {
			return fmt.Errorf("updating %s status to %s: %s", pbc.Name, pbc.Status.State, err)
		}
	case api.BundleControllerStateUpgradeAvailable:
		if !latest {
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
		if pbc.Spec.ActiveBundle == "" {
			pbc.Spec.ActiveBundle = latestBundle.Name
		}
		pbc.Status.State = api.BundleControllerStateActive
		m.log.V(6).Info("update", "PackageBundleController", pbc.Name, "state", pbc.Status.State)
		err = m.bundleClient.Save(ctx, pbc)
		if err != nil {
			return fmt.Errorf("updating %s status to %s: %s", pbc.Name, pbc.Status.State, err)
		}
	}

	return nil
}
