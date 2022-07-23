package main

import (
	"ecrtokencronjob/src/aws"
	k8s "ecrtokencronjob/src/kubernetes"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	envVarAwsSecret       = "DOCKER_SECRET_NAME"
	envVarTargetNamespace = "TARGET_NAMESPACE"
	envVarRegistries      = "DOCKER_REGISTRIES"
	envVarIRSAToken       = "AWS_WEB_IDENTITY_TOKEN_FILE"
)

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	fmt.Println("Running at " + time.Now().UTC().String())

	name := os.Getenv(envVarAwsSecret)
	if name == "" {
		panic(fmt.Sprintf("Environment variable %s is required", envVarAwsSecret))
	}

	// Check if IRSA is setup
	// If IRSA is enabled, use IRSA to setup enviroment variables for AWS Creds
	webIdentityTokenFile := os.Getenv(envVarIRSAToken)
	if webIdentityTokenFile != "" {
		err := aws.SetupIRSA()
		checkErr(err)
	}

	fmt.Println("Fetching auth data from AWS... ")
	credentials, err := aws.GetDockerCredentials()
	checkErr(err)
	fmt.Println("Success.")

	servers := getServerList(credentials.Server)
	fmt.Printf("Docker Registries: %s\n", strings.Join(servers, ","))

	namespaces, err := k8s.GetNamespaces(os.Getenv(envVarTargetNamespace))
	checkErr(err)
	fmt.Printf("Updating kubernetes secret [%s] in %d namespaces\n", name, len(namespaces))

	failed := false
	for _, ns := range namespaces {
		fmt.Printf("Updating secret in namespace [%s]... ", ns)
		err = k8s.UpdatePassword(ns, name, credentials.Username, credentials.Password, servers)
		if nil != err {
			fmt.Printf("failed: %s\n", err)
			failed = true
		} else {
			fmt.Println("success")
		}
	}

	if failed {
		panic(fmt.Sprintf("failed to create one of more Docker login secrets"))
	}

	fmt.Println("Job complete.")
}

func getServerList(defaultServer string) []string {
	addedServersSetting := os.Getenv(envVarRegistries)

	if addedServersSetting == "" {
		return []string{defaultServer}
	}

	return strings.Split(addedServersSetting, ",")
}
