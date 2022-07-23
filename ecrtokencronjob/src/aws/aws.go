package aws

import (
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/sts"
	"os"
	"strings"
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
		panic(fmt.Sprint("Enviroment variable %s missing, check that Webhook for IRSA is setup", envRoleARN))
	}

	webIdentityTokenFile := os.Getenv(envWebTokenFile)
	if webIdentityTokenFile == "" {
		panic(fmt.Sprint("Enviroment variable %s missing, check that token is mounted", envWebTokenFile))
	}
	token, err := os.ReadFile(webIdentityTokenFile)
	if err != nil {
		panic(err)
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
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case sts.ErrCodeMalformedPolicyDocumentException:
				panic(fmt.Sprint(sts.ErrCodeMalformedPolicyDocumentException, aerr.Error()))
			case sts.ErrCodePackedPolicyTooLargeException:
				panic(fmt.Sprint(sts.ErrCodePackedPolicyTooLargeException, aerr.Error()))
			case sts.ErrCodeIDPRejectedClaimException:
				panic(fmt.Sprint(sts.ErrCodeIDPRejectedClaimException, aerr.Error()))
			case sts.ErrCodeIDPCommunicationErrorException:
				panic(fmt.Sprint(sts.ErrCodeIDPCommunicationErrorException, aerr.Error()))
			case sts.ErrCodeInvalidIdentityTokenException:
				panic(fmt.Sprint(sts.ErrCodeInvalidIdentityTokenException, aerr.Error()))
			case sts.ErrCodeExpiredTokenException:
				panic(fmt.Sprint(sts.ErrCodeExpiredTokenException, aerr.Error()))
			case sts.ErrCodeRegionDisabledException:
				panic(fmt.Sprint(sts.ErrCodeRegionDisabledException, aerr.Error()))
			default:
				panic(fmt.Sprint(aerr.Error()))
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			panic(fmt.Sprint(err.Error()))
		}
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
