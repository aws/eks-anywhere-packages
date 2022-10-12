package main

import (
	"log"
	"os"
	"time"

	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/aws"
	k8s "github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/kubernetes"
)

const (
	envVarAwsSecret = "ECR_TOKEN_SECRET_NAME"       //#nosec G101
	envVarIRSAToken = "AWS_WEB_IDENTITY_TOKEN_FILE" //#nosec G101
)

func checkErrAndLog(err error, logger *log.Logger) {
	if err != nil {
		logger.Println(err)
		os.Exit(0)
	}
}

func main() {
	infoLogger := log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	warningLogger := log.New(os.Stderr, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
	errorLogger := log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	infoLogger.Println("Running at " + time.Now().UTC().String())

	secretname := os.Getenv(envVarAwsSecret)
	if secretname == "" {
		errorLogger.Printf("Environment variable %s is required", envVarAwsSecret)
		os.Exit(0)
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

	err, failedList := k8s.UpdateTokens(secretname, credentials)
	if len(failedList) > 0 {
		warningLogger.Printf("Failed the following: %s", failedList)
	}
	checkErrAndLog(err, errorLogger)

	infoLogger.Println("Job complete at " + time.Now().UTC().String())
}
