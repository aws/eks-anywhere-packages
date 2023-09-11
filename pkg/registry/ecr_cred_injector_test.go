package registry_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"

	ctrlmocks "github.com/aws/eks-anywhere-packages/controllers/mocks"
	"github.com/aws/eks-anywhere-packages/pkg/registry"
)

func TestECRCredInjector(t *testing.T) {
	gomockController := gomock.NewController(t)
	k8sClient := ctrlmocks.NewMockClient(gomockController)
	injector, err := registry.NewECRCredInjector(context.Background(), k8sClient, logr.Discard())
	if err != nil {
		t.Errorf("should not have failed to create injector: %v", err)
	}
	err = injector.Refresh(context.Background())
	if err == nil {
		t.Error("refresh should have failed because no AWS credential has been set")
	}
}

func TestExtractECRToken(t *testing.T) {
	auth, err := registry.ExtractECRToken(base64.StdEncoding.EncodeToString([]byte("username:password")))
	if err != nil {
		t.Errorf("encode should not fail: %v", err)
	}
	if auth.Username != "username" {
		t.Errorf("username is not expected")
	}
	if auth.Password != "password" {
		t.Errorf("password is not expected")
	}
}

func TestIsECRRegistry(t *testing.T) {
	res := registry.IsECRRegistry("5551212.dkr.ecr.us-west-2.amazonaws.com")
	if res != true {
		t.Errorf("registry is expected to be ECR")
	}
	res = registry.IsECRRegistry("localhost")
	if res != false {
		t.Errorf("registry is not ECR")
	}
}
