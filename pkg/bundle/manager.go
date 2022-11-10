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

func (m bundleManager) ProcessBundle(_ context.Context, newBundle *api.PackageBundle) (bool, error) {
	if newBundle.Namespace != api.PackageNamespace {
		if newBundle.Status.State != api.PackageBundleStateIgnored {
			newBundle.Status.State = api.PackageBundleStateIgnored
			m.log.V(6).Info("update", "bundle", newBundle.Name, "state", newBundle.Status.State)
			return true, nil
		}
		return false, nil
	}

	if !newBundle.IsValidVersion() {
		if newBundle.Status.State != api.PackageBundleStateInvalid {
			newBundle.Status.State = api.PackageBundleStateInvalid
			m.log.V(6).Info("update", "bundle", newBundle.Name, "state", newBundle.Status.State)
			return true, nil
		}
		return false, nil
	}

	if newBundle.Status.State != api.PackageBundleStateAvailable {
		newBundle.Status.State = api.PackageBundleStateAvailable
		m.log.V(6).Info("update", "bundle", newBundle.Name, "state", newBundle.Status.State)
		return true, nil
	}
	return false, nil
}

func (m *bundleManager) hasBundleNamed(bundles []api.PackageBundle, bundleName string) bool {
	for _, b := range bundles {
		if b.Name == bundleName {
			return true
		}
	}
	return false
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

	latestBundle, err := m.registryClient.LatestBundle(ctx, pbc.GetBundleURI(), info.Major, info.Minor)
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

	allBundles, err := m.bundleClient.GetBundleList(ctx)
	if err != nil {
		return fmt.Errorf("getting bundle list: %s", err)
	}

	if !m.hasBundleNamed(allBundles, latestBundle.Name) {
		err = m.bundleClient.CreateBundle(ctx, latestBundle)
		if err != nil {
			return err
		}
	}
	latestBundleIsCurrentBundle := latestBundle.Name == pbc.Spec.ActiveBundle

	switch pbc.Status.State {
	case api.BundleControllerStateActive:
		err = m.bundleClient.CreateClusterNamespace(ctx, pbc.Name)
		if err != nil {
			return fmt.Errorf("creating namespace for %s: %s", pbc.Name, err)
		}

		err = m.bundleClient.CreateClusterConfigMap(ctx, pbc.Name)
		if err != nil {
			return fmt.Errorf("creating configmap for %s: %s", pbc.Name, err)
		}

		err = m.targetClient.CreateClusterNamespace(ctx, pbc.GetName())
		if err != nil {
			return fmt.Errorf("creating workload cluster namespace eksa-packages for %s: %s", pbc.Name, err)
		}

		if len(pbc.Spec.ActiveBundle) > 0 {

			if !m.hasBundleNamed(allBundles, pbc.Spec.ActiveBundle) {

				activeBundle, err := m.registryClient.DownloadBundle(ctx, pbc.GetActiveBundleURI())
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
			if pbc.Status.Detail != latestBundle.Name+" available" {
				pbc.Status.Detail = latestBundle.Name + " available"
				err = m.bundleClient.SaveStatus(ctx, pbc)
				if err != nil {
					return fmt.Errorf("updating %s detail to %s: %s", pbc.Name, pbc.Status.Detail, err)
				}
			}
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
