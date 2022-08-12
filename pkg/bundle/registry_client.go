package bundle

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/artifacts"
)

type RegistryClient interface {
	// LatestBundle pulls the bundle tagged with "latest" from the bundle source.
	LatestBundle(ctx context.Context, baseRef string, kubeVersion string) (
		*api.PackageBundle, error)

	// DownloadBundle downloads the bundle with a given tag.
	DownloadBundle(ctx context.Context, ref string) (
		*api.PackageBundle, error)
}

type registryClient struct {
	puller artifacts.Puller
}

func NewRegistryClient(puller artifacts.Puller) (manager *registryClient) {
	return &registryClient{
		puller: puller,
	}
}

var _ RegistryClient = (*registryClient)(nil)

// LatestBundle pulls the bundle tagged with "latest" from the bundle source.
//
// It returns an error if the bundle it retrieves is empty. This is because an
// empty file would be successfully parsed and a Zero-value PackageBundle
// returned, which is not acceptable.
func (rc *registryClient) LatestBundle(ctx context.Context, baseRef string, kubeVersion string) (*api.PackageBundle, error) {
	tag := "latest"
	kubeVersion = strings.TrimPrefix(kubeVersion, "v")
	kubeVersionSplit := strings.Split(kubeVersion, ".")
	if len(kubeVersionSplit) < 2 {
		return nil, fmt.Errorf("kubeversion should be in <major>.<minor> format")
	}
	major, minor := kubeVersionSplit[0], kubeVersionSplit[1]
	ref := fmt.Sprintf("%s:v%s-%s-%s", baseRef, major, minor, tag)
	return rc.DownloadBundle(ctx, ref)
}

func (rc *registryClient) DownloadBundle(ctx context.Context, ref string) (*api.PackageBundle, error) {
	data, err := rc.puller.Pull(ctx, ref)
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
