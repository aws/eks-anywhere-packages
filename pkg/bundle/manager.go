package bundle

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/authenticator"
)

type Manager interface {
	// ProcessBundle returns true if there are changes
	ProcessBundle(ctx context.Context, newBundle *api.PackageBundle) (bool, error)

	// ProcessBundleController process the bundle controller
	ProcessBundleController(ctx context.Context, pbc *api.PackageBundleController) error
}

type bundleManager struct {
	log            logr.Logger
	bundleClient   Client
	registryClient RegistryClient
	targetClient   authenticator.TargetClusterClient
}

func NewBundleManager(log logr.Logger, registryClient RegistryClient, bundleClient Client, targetClient authenticator.TargetClusterClient) *bundleManager {
	return &bundleManager{
		log:            log,
		bundleClient:   bundleClient,
		registryClient: registryClient,
		targetClient:   targetClient,
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

func (m *bundleManager) ProcessBundleController(ctx context.Context, pbc *api.PackageBundleController) error {
	info, err := m.targetClient.GetServerVersion(ctx, pbc.Name)
	if err != nil {
		m.log.Error(err, "Unable to get server version")
		if pbc.Status.State == api.BundleControllerStateActive || pbc.Status.State == "" {
			pbc.Status.Detail = err.Error()
			pbc.Status.State = api.BundleControllerStateDisconnected
			err = m.bundleClient.SaveStatus(ctx, pbc)
			if err != nil {
				return fmt.Errorf("updating %s status to %s: %s", pbc.Name, pbc.Status.State, err)
			}
		}
		return nil
	}

	latestBundle, err := m.registryClient.LatestBundle(ctx, pbc.GetBundleURI(), info.String())
	if err != nil {
		m.log.Error(err, "Unable to get latest bundle")
		if pbc.Status.State == api.BundleControllerStateActive || pbc.Status.State == "" {
			pbc.Status.State = api.BundleControllerStateDisconnected
			pbc.Status.Detail = err.Error()
			err = m.bundleClient.SaveStatus(ctx, pbc)
			if err != nil {
				return fmt.Errorf("updating %s status to %s: %s", pbc.Name, pbc.Status.State, err)
			}
		}
		return nil
	}

	serverVersion := fmt.Sprintf("v%s-%s", info.Major, info.Minor)
	m.log.V(6).Info("info.String()", "version", serverVersion)
	sortedBundles, err := m.bundleClient.GetBundleList(ctx, serverVersion)
	if err != nil {
		return fmt.Errorf("getting bundle list: %s", err)
	}

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
			return err
		}
	}

	switch pbc.Status.State {
	case api.BundleControllerStateActive:
		err = m.bundleClient.CreateClusterNamespace(ctx, pbc.Name)
		if err != nil {
			return fmt.Errorf("creating namespace for %s: %s", pbc.Name, err)
		}

		if len(pbc.Spec.ActiveBundle) > 0 {
			activeBundle, err := m.bundleClient.GetBundle(ctx, pbc.Spec.ActiveBundle)
			if err != nil {
				m.log.Error(err, "Unable to get active bundle", "bundle", pbc.Spec.ActiveBundle)
				return nil
			}

			if activeBundle == nil {

				activeBundle, err = m.registryClient.DownloadBundle(ctx, pbc.GetActiveBundleURI())
				if err != nil {
					m.log.Error(err, "Active bundle download failed", "bundle", pbc.Spec.ActiveBundle)
					return nil
				}
				m.log.Info("Bundle downloaded", "bundle", pbc.Spec.ActiveBundle)

				err = m.bundleClient.CreateBundle(ctx, activeBundle)
				if err != nil {
					m.log.Error(err, "Recreate active bundle failed", "bundle", pbc.Spec.ActiveBundle)
					return nil
				}
				m.log.Info("Bundle created", "bundle", pbc.Spec.ActiveBundle)
			}
		}

		if latestBundleIsCurrentBundle {
			break
		}
		pbc.Status.State = api.BundleControllerStateUpgradeAvailable
		m.log.V(6).Info("update", "PackageBundleController", pbc.Name, "state", pbc.Status.State)
		pbc.Status.Detail = latestBundle.Name + " available"
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
		pbc.Status.Detail = ""
		err = m.bundleClient.SaveStatus(ctx, pbc)
		if err != nil {
			return fmt.Errorf("updating %s status to %s: %s", pbc.Name, pbc.Status.State, err)
		}
	case api.BundleControllerStateDisconnected:
		pbc.Status.State = api.BundleControllerStateActive
		m.log.V(6).Info("update", "PackageBundleController", pbc.Name, "state", pbc.Status.State)
		pbc.Status.Detail = ""
		err = m.bundleClient.SaveStatus(ctx, pbc)
		if err != nil {
			return fmt.Errorf("updating %s status to %s: %s", pbc.Name, pbc.Status.State, err)
		}
	default:
		if pbc.Spec.ActiveBundle != "" {
			pbc.Status.State = api.BundleControllerStateActive
			m.log.V(6).Info("update", "PackageBundleController", pbc.Name, "state", pbc.Status.State)
			pbc.Status.Detail = ""
			err = m.bundleClient.SaveStatus(ctx, pbc)
			if err != nil {
				return fmt.Errorf("updating %s status to %s: %s", pbc.Name, pbc.Status.State, err)
			}
		} else {
			pbc.Spec.ActiveBundle = latestBundle.Name
			m.log.V(6).Info("update", "PackageBundleController", pbc.Name, "activeBundle", pbc.Spec.ActiveBundle)
			pbc.Status.Detail = ""
			err = m.bundleClient.Save(ctx, pbc)
			if err != nil {
				return fmt.Errorf("updating %s activeBundle to %s: %s", pbc.Name, pbc.Spec.ActiveBundle, err)
			}
		}
	}

	return nil
}
