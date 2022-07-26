package aws

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/sts"
)

type DockerCredentials struct {
	Username, Password, Server string
}

const (
	envRoleARN         = "AWS_ROLE_ARN"
	envWebTokenFile    = "AWS_WEB_IDENTITY_TOKEN_FILE"
	sessionName        = "dockercreds"
	sessionTimeSeconds = 1000
)

func GetDockerCredentials() (*DockerCredentials, error) {
	svc := ecr.New(session.Must(session.NewSession()))
	token, err := svc.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{})
	if nil != err {
		return nil, err
	}

	// We expect the response to always be a single entry
	auth := token.AuthorizationData[0]

	decode, err := base64.StdEncoding.DecodeString(*auth.AuthorizationToken)
	if nil != err {
		return nil, err
	}

	parts := strings.Split(string(decode), ":")
	cred := DockerCredentials{
		Username: parts[0],
		Password: parts[1],
		Server:   *auth.ProxyEndpoint,
	}

	return &cred, nil
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
	token, err := os.ReadFile(webIdentityTokenFile)
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
