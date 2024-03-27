package awscred

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

const (
	configSecretPath          = "/secrets/aws-creds/config"
	accessKeySecretPath       = "/secrets/aws-creds/AWS_ACCESS_KEY_ID"
	secretAccessKeySecretPath = "/secrets/aws-creds/AWS_SECRET_ACCESS_KEY"
	regionSecretPath          = "/secrets/aws-creds/REGION"
	createConfigPath          = "/config"
)

func generateAwsConfigSecret(accessKeyPath, secretAccessKeyPath, regionPath string) (string, error) {
	accessKeyByte, err := ioutil.ReadFile(accessKeyPath)
	if err != nil {
		return "", err
	}
	accessKey := strings.Trim(string(accessKeyByte), "'")
	secretAccessKeyByte, err := ioutil.ReadFile(secretAccessKeyPath)
	if err != nil {
		return "", err
	}
	secretAccessKey := strings.Trim(string(secretAccessKeyByte), "'")
	regionByte, err := ioutil.ReadFile(regionPath)
	if err != nil {
		return "", err
	}
	region := strings.Trim(string(regionByte), "'")

	awsConfig := fmt.Sprintf(
		`
[default]
aws_access_key_id=%s
aws_secret_access_key=%s
region=%s
`, accessKey, secretAccessKey, region)

	return awsConfig, err
}

func GetAwsConfigPath() (string, error) {
	_, err := os.Stat(configSecretPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			awsConfig, err := generateAwsConfigSecret(accessKeySecretPath, secretAccessKeySecretPath, regionSecretPath)
			err = ioutil.WriteFile(createConfigPath, []byte(awsConfig), 0o400)
			return createConfigPath, err
		}
	}
	return configSecretPath, nil
}
