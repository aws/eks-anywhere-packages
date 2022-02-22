package testutil

import (
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	clienttesting "k8s.io/client-go/testing"
)

type fakeDiscovery struct {
	*fakediscovery.FakeDiscovery
}

func NewFakeDiscovery(major, minor string) *fakeDiscovery {
	fake := &fakediscovery.FakeDiscovery{
		Fake: &clienttesting.Fake{},
		FakedServerVersion: &version.Info{
			Major: major,
			Minor: minor,
		},
	}
	return &fakeDiscovery{FakeDiscovery: fake}
}

func NewFakeDiscoveryWithDefaults() *fakeDiscovery {
	return NewFakeDiscovery("1", "21+")
}
