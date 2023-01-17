package aws

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/sts"
	"k8s.io/client-go/kubernetes"

	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets/common"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/utils"
)

type AwsSecret struct {
	secretName       string
	defaultClientSet *kubernetes.Clientset
	remoteClientSets secrets.RemoteClusterClientset
}

var _ secrets.Secret = (*AwsSecret)(nil)

const (
	envVarAwsSecret      = "ECR_TOKEN_SECRET_NAME"       //#nosec G101
	envVarIRSAToken      = "AWS_WEB_IDENTITY_TOKEN_FILE" //#nosec G101
	envRoleARN           = "AWS_ROLE_ARN"
	envWebTokenFile      = "AWS_WEB_IDENTITY_TOKEN_FILE" //#nosec G101
	sessionName          = "GetECRTOKENSession"
	sessionTimeSeconds   = 1000
	defaultAccountID     = "783794618700"
	devAccountID         = "857151390494"
	envRegionName        = "AWS_REGION"
	envVarAwsAccessKeyID = "AWS_ACCESS_KEY_ID"
	envVarAwsAccessKey   = "AWS_SECRET_ACCESS_KEY"
	envSessionToken      = "AWS_SESSION_TOKEN"
	regionDefault        = "us-west-2"
)

func (aws *AwsSecret) Init(defaultClientSet *kubernetes.Clientset, remoteClientSets secrets.RemoteClusterClientset) error {
	secretname := os.Getenv(envVarAwsSecret)
	if secretname == "" {
		return fmt.Errorf("environment variable %s is required", envVarAwsSecret)
	}
	aws.secretName = secretname

	// Check if IRSA is setup
	// If IRSA is enabled, use IRSA to setup enviroment variables for AWS Creds
	webIdentityTokenFile := os.Getenv(envVarIRSAToken)
	if webIdentityTokenFile != "" {
		err := setupIRSA()
		if err != nil {
			return err
		}
	}

	aws.defaultClientSet = defaultClientSet
	aws.remoteClientSets = remoteClientSets
	return nil
}

func (aws *AwsSecret) IsActive() bool {
	if val, _ := os.LookupEnv(envVarAwsAccessKeyID); val == "" {
		return false
	}
	if val, _ := os.LookupEnv(envVarAwsAccessKey); val == "" {
		return false
	}
	return true
}

func (aws *AwsSecret) GetCredentials() ([]*secrets.Credential, error) {
	utils.InfoLogger.Println("fetching auth data from AWS... ")
	// Default AWS Region to us-west-2 if not set by User.
	_, ok := os.LookupEnv(envRegionName)
	if !ok {
		err := os.Setenv(envRegionName, regionDefault)
		if err != nil {
			return nil, err
		}
	}

	var ecrRegs []*string
	defID := defaultAccountID
	ecrRegs = append(ecrRegs, &defID)
	devID := devAccountID
	ecrRegs = append(ecrRegs, &devID)
	svc := ecr.New(session.Must(session.NewSession()))
	token, err := svc.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{RegistryIds: ecrRegs})
	if err != nil {
		return nil, err
	}

	if token == nil {
		return nil, fmt.Errorf("response output from ECR was nil")
	}

	if len(token.AuthorizationData) == 0 {
		return nil, fmt.Errorf("authorization data was empty")
	}

	var creds []*secrets.Credential
	for _, auth := range token.AuthorizationData {
		decode, err := base64.StdEncoding.DecodeString(*auth.AuthorizationToken)
		if err != nil {
			return nil, err
		}

		parts := strings.Split(string(decode), ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("error parsing username and password from authorization token")
		}
		cred := secrets.Credential{
			Username: parts[0],
			Password: parts[1],
			Registry: *auth.ProxyEndpoint,
		}
		creds = append(creds, &cred)
	}

	utils.InfoLogger.Println("success.")
	return creds, nil
}

func (aws *AwsSecret) BroadcastCredentials() error {
	creds, err := aws.GetCredentials()
	if err != nil {
		return err
	}
	dockerConfig := common.CreateDockerAuthConfig(creds)
	return common.BroadcastDockerAuthConfig(dockerConfig, &aws.remoteClientSets, aws.secretName)
}

func setupIRSA() error {
	roleArn := os.Getenv(envRoleARN)
	if roleArn == "" {
		return fmt.Errorf("environment variable %s missing, check that Webhook for IRSA is setup", envRoleARN)
	}

	webIdentityTokenFile := os.Getenv(envWebTokenFile)
	if webIdentityTokenFile == "" {
		return fmt.Errorf("environment variable %s missing, check that token is mounted", envWebTokenFile)
	}

	token, err := os.ReadFile(filepath.Clean(webIdentityTokenFile))
	if err != nil {
		return err
	}
	webIdentityToken := string(token)

	session, err := session.NewSession()
	if err != nil {
		return err
	}
	svc := sts.New(session)
	input := &sts.AssumeRoleWithWebIdentityInput{
		DurationSeconds:  aws.Int64(sessionTimeSeconds),
		RoleArn:          aws.String(roleArn),
		RoleSessionName:  aws.String(sessionName),
		WebIdentityToken: aws.String(webIdentityToken),
	}
	result, err := svc.AssumeRoleWithWebIdentity(input)
	if err != nil {
		return err
	}

	err = os.Setenv(envVarAwsAccessKeyID, aws.StringValue(result.Credentials.AccessKeyId))
	if err != nil {
		return err
	}
	err = os.Setenv(envVarAwsAccessKey, aws.StringValue(result.Credentials.SecretAccessKey))
	if err != nil {
		return err
	}
	err = os.Setenv(envSessionToken, aws.StringValue(result.Credentials.SessionToken))
	if err != nil {
		return err
	}

	return err
}
