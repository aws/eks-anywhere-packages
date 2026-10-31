package main

import (
	"testing"

	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets/aws"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets/registrymirror"
)

func TestRefreshableSecretsRefreshesRegistryMirrorBeforeAWS(t *testing.T) {
	got := refreshableSecrets()
	if len(got) != 2 {
		t.Fatalf("refreshableSecrets() returned %d secrets, want 2", len(got))
	}
	if _, ok := got[0].(*registrymirror.RegistryMirrorSecret); !ok {
		t.Errorf("refreshableSecrets()[0] = %T, want *registrymirror.RegistryMirrorSecret", got[0])
	}
	if _, ok := got[1].(*aws.AwsSecret); !ok {
		t.Errorf("refreshableSecrets()[1] = %T, want *aws.AwsSecret", got[1])
	}
}
