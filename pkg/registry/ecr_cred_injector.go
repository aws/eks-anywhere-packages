package registry

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	awsCredentials "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/docker/cli/cli/config"
	dockerTypes "github.com/docker/cli/cli/config/types"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"oras.land/oras-go/v2/registry/remote/auth"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/authenticator"
)

const (
	awsSecretPath = "/tmp/config/aws-secret" //#nosec G101
)

// ECRCredInjector is an adapter to convert ECR credential to Docker credential. Since the converted docker credential is only valid for 12 hours, this adapter is constantly running. It's responsibility is to make sure docker config in the filesystem contains ECR credential to pull bundle yaml and charts.
type ECRCredInjector struct {
	k8sClient client.Client
	ecrClient *ecr.Client
	log       logr.Logger
}

func NewECRCredInjector(ctx context.Context, k8sClient client.Client, log logr.Logger) (*ECRCredInjector, error) {
	l := log.WithName("ECRCredInjector")
	ecrClient, err := GetECRClient(ctx, l)
	if err != nil {
		return nil, err
	}

	return &ECRCredInjector{
		k8sClient: k8sClient,
		ecrClient: ecrClient,
		log:       l,
	}, nil
}

func (a *ECRCredInjector) Run(ctx context.Context) {
	err := a.Refresh(ctx)
	if err != nil {
		a.log.Error(err, "Failed to inject ECR credential to docker config")
	} else {
		a.log.Info("ECR credential is injected to the docker config file")
	}

	for range time.Tick(time.Hour) {
		err := a.Refresh(ctx)
		if err != nil {
			a.log.Error(err, "Failed to refresh ECR credential in dockerconfig file")
		} else {
			a.log.Info("injected ECR credential has be refreshed")
		}
	}
}

func (a *ECRCredInjector) Refresh(ctx context.Context) error {
	a.log.Info("Refreshing ECR credential")
	cred, err := GetCredential(a.ecrClient)
	if err != nil {
		return err
	}

	dockerSecret, err := a.GetRegistryMirrorSecret(ctx)
	if err != nil {
		return err
	}

	registry, err := a.GetECR(ctx)
	if err != nil {
		return err
	}

	if !IsECRRegistry(registry) {
		a.log.Info("defaultRegistry is not ECR registry, skip injecting credential to docker config")
		return nil
	}
	// update "config.json" in dockerSecret
	return a.InjectCredential(ctx, *dockerSecret, registry, cred)
}

// GetECR get the defaultRegistry config from package bundle controller.
func (a *ECRCredInjector) GetECR(ctx context.Context) (string, error) {
	pbc := &api.PackageBundleController{}
	err := a.k8sClient.Get(ctx, types.NamespacedName{
		Namespace: api.PackageNamespace,
		Name:      os.Getenv("CLUSTER_NAME"),
	}, pbc)
	if err != nil {
		return "", err
	}

	// defaultRegistry could be followed by path
	ss := strings.Split(pbc.Spec.DefaultRegistry, "/")
	return ss[0], nil
}

// GetRegistryMirrorSecret gets registry mirror secret from eksa-packages namespace
func (a *ECRCredInjector) GetRegistryMirrorSecret(ctx context.Context) (*v1.Secret, error) {
	var secret v1.Secret

	err := a.k8sClient.Get(ctx, types.NamespacedName{
		Namespace: api.PackageNamespace,
		// this secret is populated by token refresher
		Name: authenticator.MirrorCredName,
	}, &secret)
	if err != nil {
		return nil, err
	}

	return &secret, nil
}

// InjectCredential update field "config.json" in the secret, which is used by packages controller's oras and helm
func (a *ECRCredInjector) InjectCredential(ctx context.Context, secret v1.Secret, registry string, cred auth.Credential) error {
	d := secret.Data["config.json"]
	var configJson []byte
	if _, err := base64.StdEncoding.Decode(d, configJson); err != nil {
		return err
	}

	dockerConfig, err := config.LoadFromReader(strings.NewReader(string(configJson)))
	if err != nil {
		return err
	}
	dockerConfig.AuthConfigs[registry] = dockerTypes.AuthConfig{
		Username: cred.Username,
		Password: cred.Password,
	}

	buf := new(bytes.Buffer)
	err = dockerConfig.SaveToWriter(buf)
	if err != nil {
		return err
	}

	secret.Data["config.json"] = buf.Bytes()
	return a.k8sClient.Update(ctx, &secret, &client.UpdateOptions{})
}

func GetECRClient(ctx context.Context, log logr.Logger) (*ecr.Client, error) {
	var c *ecr.Client
	var err error
	c = getECRClientFromConfig(ctx, log)
	if c == nil {
		c, err = getECRClientFromVariables(ctx, log)
		if err != nil {
			return nil, fmt.Errorf("Unable to load AWS config, " + err.Error())
		}
	}
	return c, nil
}

// getECRClientFromConfig tries to get ecrClient from aws config file, if failed, return nil
func getECRClientFromConfig(ctx context.Context, log logr.Logger) *ecr.Client {
	configPath := awsSecretPath + "/config"
	// check if configPath exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Info("aws config file does not exist, skip loading config")
		return nil
	}

	cfg, err := awsConfig.LoadDefaultConfig(ctx,
		awsConfig.WithSharedConfigFiles([]string{configPath}),
	)
	if err != nil {
		fmt.Println("Unable to load AWS config from file, " + err.Error())
		return nil
	}

	return ecr.NewFromConfig(cfg)
}

// getECRClientFromVariables tries to get ecrClient from access_key and region
func getECRClientFromVariables(ctx context.Context, log logr.Logger) (*ecr.Client, error) {
	// similar to https://github.com/aws/eks-anywhere-packages/blob/eca65837c277f7769f721f2251b3e92f0d8edb68/credentialproviderpackage/pkg/awscred/awscred.go#L11
	accessKeyPath := awsSecretPath + "/AWS_ACCESS_KEY_ID"
	secretAccessKeyPath := awsSecretPath + "/AWS_SECRET_ACCESS_KEY"
	regionPath := awsSecretPath + "/REGION"

	accessKeyByte, err := os.ReadFile(accessKeyPath)
	if err != nil {
		log.Error(err, "Cannot get access key from file")
	}
	accessKey := strings.Trim(string(accessKeyByte), "'")
	secretAccessKeyByte, err := os.ReadFile(secretAccessKeyPath)
	if err != nil {
		log.Error(err, "Cannot get secret access key from file")
	}
	secretAccessKey := strings.Trim(string(secretAccessKeyByte), "'")
	regionByte, err := os.ReadFile(regionPath)
	if err != nil {
		log.Error(err, "Cannot get region from file, %v")
	}
	region := strings.Trim(string(regionByte), "'")

	cfg, err := awsConfig.LoadDefaultConfig(ctx,
		awsConfig.WithCredentialsProvider(awsCredentials.NewStaticCredentialsProvider(accessKey, secretAccessKey, "")),
		awsConfig.WithRegion(region),
	)
	if err != nil {
		return nil, err
	}

	return ecr.NewFromConfig(cfg), nil
}

func GetCredential(ecrClient *ecr.Client) (auth.Credential, error) {
	out, err := ecrClient.GetAuthorizationToken(context.Background(), &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return auth.EmptyCredential, err
	}
	token := out.AuthorizationData[0].AuthorizationToken

	cred, err := ExtractECRToken(aws.ToString(token))
	if err != nil {
		return auth.EmptyCredential, err
	}

	return *cred, nil
}

func ExtractECRToken(token string) (*auth.Credential, error) {
	decodedToken, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	parts := strings.SplitN(string(decodedToken), ":", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid token: expected two parts, got %d", len(parts))
	}

	return &auth.Credential{
		Username: parts[0],
		Password: parts[1],
	}, nil
}

func IsECRRegistry(registry string) bool {
	return strings.HasSuffix(registry, "amazonaws.com")
}
