package registry_test

import (
	"context"
	"crypto/x509"
	"fmt"
	"testing"

	"github.com/docker/cli/cli/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry/remote"

	"github.com/aws/eks-anywhere-packages/pkg/registry"
)

var (
	ctx   = context.Background()
	image = registry.Artifact{
		Registry:   "public.ecr.aws",
		Repository: "eks-anywhere/eks-anywhere-packages",
		Digest:     "sha256:6efe21500abbfbb6b3e37b80dd5dea0b11a0d1b145e84298fee5d7784a77e967",
		Tag:        "0.2.22-eks-a-24",
	}
	certificates = &x509.CertPool{}
)

func TestOCIRegistryClient_Destination(t *testing.T) {
	sc := newStorageContext(t, "")
	sut := registry.NewOCIRegistry(sc, newTestRegistry(t, image.Registry))
	destination := sut.Destination(image)
	assert.Equal(t, "localhost/eks-anywhere/eks-anywhere-packages@sha256:6efe21500abbfbb6b3e37b80dd5dea0b11a0d1b145e84298fee5d7784a77e967", destination)
	sut.SetProject("project/")
	destination = sut.Destination(image)
	assert.Equal(t, "localhost/project/eks-anywhere/eks-anywhere-packages@sha256:6efe21500abbfbb6b3e37b80dd5dea0b11a0d1b145e84298fee5d7784a77e967", destination)
}

func TestOCIRegistryClient_GetStorage(t *testing.T) {
	sc := newStorageContext(t, "")
	sut := registry.NewOCIRegistry(sc, newTestRegistry(t, image.Registry))
	_, err := sut.GetStorage(context.Background(), image)
	assert.NoError(t, err)

	bogusImage := registry.Artifact{
		Registry:   "localhost",
		Repository: "!@#$",
		Digest:     "sha256:6efe21500abbfbb6b3e37b80dd5dea0b11a0d1b145e84298fee5d7784a77e967",
	}
	_, err = sut.GetStorage(context.Background(), bogusImage)
	assert.EqualError(t, err, fmt.Sprintf("error creating repository %[1]s: invalid reference: invalid repository %[1]q", bogusImage.Repository))
}

func newStorageContext(t *testing.T, dir string) registry.StorageContext {
	configFile, err := config.Load(dir)
	require.NoError(t, err)
	store := registry.NewDockerCredentialStore(configFile)
	return registry.NewStorageContext("localhost", store, certificates, false)
}

func newTestRegistry(t *testing.T, host string) *remote.Registry {
	t.Helper()
	r, err := remote.NewRegistry(host)
	require.NoError(t, err)
	return r
}
