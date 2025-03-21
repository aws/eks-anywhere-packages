package awscred

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	configSecretPath          = "/secrets/aws-creds/config"
	accessKeySecretPath       = "/secrets/aws-creds/AWS_ACCESS_KEY_ID"
	secretAccessKeySecretPath = "/secrets/aws-creds/AWS_SECRET_ACCESS_KEY"
	sessionTokenSecretPath    = "/secrets/aws-creds/AWS_SESSION_TOKEN"
	regionSecretPath          = "/secrets/aws-creds/REGION"
	createConfigPath          = "/config"
)

func generateAwsConfigSecret(accessKeyPath, secretAccessKeyPath, sessionTokenPath, regionPath string) (string, error) {
	accessKeyByte, err := os.ReadFile(accessKeyPath)
	if err != nil {
		return "", err
	}
	accessKey := strings.Trim(string(accessKeyByte), "'")
	secretAccessKeyByte, err := os.ReadFile(secretAccessKeyPath)
	if err != nil {
		return "", err
	}
	secretAccessKey := strings.Trim(string(secretAccessKeyByte), "'")
	regionByte, err := os.ReadFile(regionPath)
	if err != nil {
		return "", err
	}
	region := strings.Trim(string(regionByte), "'")

	// check if sessionToken exists and read it
	if _, err := os.Stat(sessionTokenPath); !os.IsNotExist(err) {
		sessionTokenByte, err := os.ReadFile(sessionTokenPath)
		if err != nil {
			return "", err
		}
		sessionToken := strings.Trim(string(sessionTokenByte), "'")

		return fmt.Sprintf(
			`
[default]
aws_access_key_id=%s
aws_secret_access_key=%s
aws_session_token=%s
region=%s
`, accessKey, secretAccessKey, sessionToken, region), nil

	}

	return fmt.Sprintf(
		`
[default]
aws_access_key_id=%s
aws_secret_access_key=%s
region=%s
`, accessKey, secretAccessKey, region), nil
}

func GetAwsConfigPath() (string, error) {
	_, err := os.Stat(configSecretPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			awsConfig, err := generateAwsConfigSecret(accessKeySecretPath, secretAccessKeySecretPath, sessionTokenSecretPath, regionSecretPath)
			err = os.WriteFile(createConfigPath, []byte(awsConfig), 0o400)
			return createConfigPath, err
		}
	}
	return configSecretPath, nil
}
