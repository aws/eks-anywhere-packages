package main

import (
	"log"
	"os"
	"strings"
	"time"

	"ecrtokenrefresher/src/aws"
	k8s "ecrtokenrefresher/src/kubernetes"
)

const (
	envVarAwsSecret       = "DOCKER_SECRET_NAME"
	envVarTargetNamespace = "TARGET_NAMESPACE"
	envVarRegistries      = "DOCKER_REGISTRIES"
	envVarIRSAToken       = "AWS_WEB_IDENTITY_TOKEN_FILE"
)

func checkErr(err error, logger *log.Logger) {
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
		checkErr(err, errorLogger)
	}

	infoLogger.Println("Fetching auth data from AWS... ")
	credentials, err := aws.GetDockerCredentials()
	checkErr(err, errorLogger)
	infoLogger.Println("Success.")

	servers := getServerList(credentials.Server)
	infoLogger.Printf("Docker Registries: %s\n", strings.Join(servers, ","))

	namespaces, err := k8s.GetNamespaces(os.Getenv(envVarTargetNamespace))
	checkErr(err, errorLogger)
	infoLogger.Printf("Updating kubernetes secret [%s] in %d namespaces\n", name, len(namespaces))

	failed := false
	for _, ns := range namespaces {
		infoLogger.Printf("Updating secret in namespace [%s]... ", ns)
		err = k8s.UpdatePassword(ns, name, credentials.Username, credentials.Password, servers)
		if nil != err {
			infoLogger.Printf("failed: %s\n", err)
			failed = true
		} else {
			infoLogger.Println("success")
		}
	}

	if failed {
		errorLogger.Fatalf("failed to create one of more Docker login secrets")
	}

	infoLogger.Println("Job complete.")
}

func getServerList(defaultServer string) []string {
	addedServersSetting := os.Getenv(envVarRegistries)

	if addedServersSetting == "" {
		return []string{defaultServer}
	}

	return strings.Split(addedServersSetting, ",")
}
