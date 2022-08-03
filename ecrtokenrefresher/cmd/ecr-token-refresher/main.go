package main

import (
	"log"
	"os"
	"time"

	"ecrtokenrefresher/pkg/aws"
	k8s "ecrtokenrefresher/pkg/kubernetes"
)

const (
	envVarAwsSecret = "ECR_TOKEN_SECRET_NAME"
	envVarIRSAToken = "AWS_WEB_IDENTITY_TOKEN_FILE"
)

func checkErrAndLog(err error, logger *log.Logger) {
	if err != nil {
		logger.Fatalln(err)
	}
}

func main() {
	infoLogger := log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	errorLogger := log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	infoLogger.Println("Running at " + time.Now().UTC().String())

	name := os.Getenv(envVarAwsSecret)
	if name == "" {
		errorLogger.Fatalf("Environment variable %s is required", envVarAwsSecret)
	}

	// Check if IRSA is setup
	// If IRSA is enabled, use IRSA to setup enviroment variables for AWS Creds
	webIdentityTokenFile := os.Getenv(envVarIRSAToken)
	if webIdentityTokenFile != "" {
		err := aws.SetupIRSA()
		checkErrAndLog(err, errorLogger)
	}

	infoLogger.Println("Fetching auth data from AWS... ")
	credentials, err := aws.GetECRCredentials()
	checkErrAndLog(err, errorLogger)
	infoLogger.Println("Success.")

	err, failedList := k8s.UpdatePasswords(name, credentials.Username, credentials.Token, credentials.Registry)
	if len(failedList) > 0 {
		errorLogger.Fatalf("Failed to update the following namespaces", failedList)
		checkErrAndLog(err, errorLogger)
	}
	checkErrAndLog(err, errorLogger)

	infoLogger.Println("Job complete at " + time.Now().UTC().String())
}
