package main

import (
	"os"
	"time"

	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets/aws"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets/common"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/secrets/registrymirror"
	"github.com/aws/eks-anywhere-packages/ecrtokenrefresher/pkg/utils"
)


func main() {
	utils.InfoLogger.Println("Running at " + time.Now().UTC().String())

	clientSet, err := common.GetDefaultClientSet()
	if err != nil {
		utils.ErrorLogger.Println(err)
		os.Exit(0)
	}
	remoteClientSets, err := common.GetRemoteClientSets(clientSet)
	if err != nil {
		utils.ErrorLogger.Println(err)
		os.Exit(0)
	}

	secrets := []secrets.Secret{
		&aws.AwsSecret{},
		&registrymirror.RegistryMirrorSecret{},
	}

	for _, secret := range secrets {
		err := secret.Init(clientSet, remoteClientSets)
		if err != nil {
			utils.ErrorLogger.Println(err)
		}
	}

	for _, secret := range secrets {
		if secret.IsActive() {
			err = secret.BroadcastCredentials()
			if err != nil {
				utils.ErrorLogger.Println(err)
			}
		}
	}

	utils.InfoLogger.Println("Job complete at " + time.Now().UTC().String())
}
