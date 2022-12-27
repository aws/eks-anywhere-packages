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
)

type ECRAuth struct {
	Username, Token, Registry string
}

const (
	envRoleARN         = "AWS_ROLE_ARN"
	envWebTokenFile    = "AWS_WEB_IDENTITY_TOKEN_FILE" //#nosec G101
	sessionName        = "GetECRTOKENSession"
	sessionTimeSeconds = 1000
	defaultAccountID   = "783794618700"
	devAccountID       = "857151390494"
	envRegionName      = "AWS_REGION"
)

func GetECRCredentials() ([]ECRAuth, []string, error) {
	var creds []ECRAuth
	var err error
	failedList := make([]string, 0)
	for _, region := range getSupportedRegions() {
		regionCreds, err := getECRCredentialByRegion(region)
		if err != nil {
			failedList = append(failedList, region)
		}
		creds = append(creds, regionCreds...)
	}
	if len(creds) < 1 {
		return nil, failedList, fmt.Errorf("failed for all regions, %v", err)
	}
	return creds, failedList, nil
}

func getSupportedRegions() []string {
	return []string{
		"us-east-1",
		"us-east-2",
		"us-west-1",
		"us-west-2",
		"af-south-1",
		"ap-east-1",
		"ap-south-2",
		"ap-southeast-3",
		"ap-south-1",
		"ap-northeast-3",
		"ap-northeast-2",
		"ap-southeast-1",
		"ap-southeast-2",
		"ap-northeast-1",
		"ca-central-1",
		"eu-central-1",
		"eu-west-1",
		"eu-west-2",
		"eu-south-1",
		"eu-west-3",
		"eu-south-2",
		"eu-north-1",
		"eu-central-2",
		"me-south-1",
		"me-central-1",
		"sa-east-1"}
}

func getECRCredentialByRegion(region string) ([]ECRAuth, error) {
	var ecrRegs []*string
	err := os.Setenv(envRegionName, region)
	if err != nil {
		return nil, err
	}
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

	var creds []ECRAuth
	for _, auth := range token.AuthorizationData {
		decode, err := base64.StdEncoding.DecodeString(*auth.AuthorizationToken)
		if err != nil {
			return nil, err
		}

		parts := strings.Split(string(decode), ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("error parsing username and password from authorization token")
		}
		cred := ECRAuth{
			Username: parts[0],
			Token:    parts[1],
			Registry: *auth.ProxyEndpoint,
		}
		creds = append(creds, cred)
	}

	return creds, nil
}

func SetupIRSA() error {
	roleArn := os.Getenv(envRoleARN)
	if roleArn == "" {
		return fmt.Errorf("Environment variable %s missing, check that Webhook for IRSA is setup", envRoleARN)
	}

	webIdentityTokenFile := os.Getenv(envWebTokenFile)
	if webIdentityTokenFile == "" {
		return fmt.Errorf("Environment variable %s missing, check that token is mounted", envWebTokenFile)
	}

	token, err := os.ReadFile(filepath.Clean(webIdentityTokenFile))
	if err != nil {
		return err
	}
	webIdentityToken := string(token)

	svc := sts.New(session.New())
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

	err = os.Setenv("AWS_ACCESS_KEY_ID", aws.StringValue(result.Credentials.AccessKeyId))
	if err != nil {
		return err
	}
	err = os.Setenv("AWS_SECRET_ACCESS_KEY", aws.StringValue(result.Credentials.SecretAccessKey))
	if err != nil {
		return err
	}
	err = os.Setenv("AWS_SESSION_TOKEN", aws.StringValue(result.Credentials.SessionToken))
	if err != nil {
		return err
	}

	return err
}
