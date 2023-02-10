package main

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/constants"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets/aws"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets/common"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets/registrymirror"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/utils"
)

func main() {
	utils.InfoLogger.Println("Running at " + time.Now().UTC().String())

	defaultClientSet, err := common.GetDefaultClientSet()
	if err != nil {
		utils.ErrorLogger.Println(err)
		os.Exit(0)
	}
	clientSets, err := common.GetRemoteClientSets(defaultClientSet)
	if err != nil {
		utils.ErrorLogger.Println(err)
		os.Exit(0)
	}

	defaultClusterName, found := os.LookupEnv(constants.DefaultClusterNameKey)
	if !found {
		utils.ErrorLogger.Println(fmt.Errorf("environment variable %s is required", constants.DefaultClusterNameKey))
		os.Exit(0)
	}
	clientSets[defaultClusterName] = defaultClientSet

	secrets := []secrets.Secret{
		&aws.AwsSecret{},
		&registrymirror.RegistryMirrorSecret{},
	}

	utils.InfoLogger.Println("Initialization starts...")
	for _, secret := range secrets {
		err := secret.Init(defaultClusterName, clientSets)
		if err != nil {
			utils.ErrorLogger.Println(err)
		}
	}
	utils.InfoLogger.Println("Initialization completes...")

	utils.InfoLogger.Println("Broadcasting starts...")
	for _, secret := range secrets {
		if secret.IsActive() {
			err = secret.BroadcastCredentials()
			if err != nil {
				utils.ErrorLogger.Println(err)
			}
		} else {
			utils.InfoLogger.Println(secret.GetName() + " is inactive")
		}
	}
	utils.InfoLogger.Println("Broadcasting ends...")

	utils.InfoLogger.Println("Job complete at " + time.Now().UTC().String())
}
