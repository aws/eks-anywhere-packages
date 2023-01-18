package registry_test

import (
	"context"
	"crypto/x509"
	"testing"

	"github.com/docker/cli/cli/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestOCIRegistryClient_Init(t *testing.T) {
	sut := registry.NewOCIRegistry(newStorageContext(t, ""))

	err := sut.Init()
	assert.NoError(t, err)

	// Does not reinitialize
	err = sut.Init()
	assert.NoError(t, err)
}

func TestOCIRegistryClient_Destination(t *testing.T) {
	sut := registry.NewOCIRegistry(newStorageContext(t, ""))
	destination := sut.Destination(image)
	assert.Equal(t, "localhost/eks-anywhere/eks-anywhere-packages@sha256:6efe21500abbfbb6b3e37b80dd5dea0b11a0d1b145e84298fee5d7784a77e967", destination)
	sut.SetProject("project/")
	destination = sut.Destination(image)
	assert.Equal(t, "localhost/project/eks-anywhere/eks-anywhere-packages@sha256:6efe21500abbfbb6b3e37b80dd5dea0b11a0d1b145e84298fee5d7784a77e967", destination)
}

func TestOCIRegistryClient_GetStorage(t *testing.T) {
	sut := registry.NewOCIRegistry(newStorageContext(t, ""))
	assert.NoError(t, sut.Init())
	_, err := sut.GetStorage(context.Background(), image)
	assert.NoError(t, err)

	bogusImage := registry.Artifact{
		Registry:   "localhost",
		Repository: "!@#$",
		Digest:     "sha256:6efe21500abbfbb6b3e37b80dd5dea0b11a0d1b145e84298fee5d7784a77e967",
	}
	_, err = sut.GetStorage(context.Background(), bogusImage)
	assert.EqualError(t, err, "error creating repository !@#$: invalid reference: invalid repository")
}

func newStorageContext(t *testing.T, dir string) registry.StorageContext {
	configFile, err := config.Load(dir)
	require.NoError(t, err)
	store := registry.NewDockerCredentialStore(configFile)
	return registry.NewStorageContext("localhost", store, certificates, false)
}
