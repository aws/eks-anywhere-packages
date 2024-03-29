package main

import (
	_ "embed"
	"os"
	"strings"

	"github.com/fsnotify/fsnotify"

	"github.com/aws/eks-anywhere-packages/credentialproviderpackage/pkg/awscred"
	cfg "github.com/aws/eks-anywhere-packages/credentialproviderpackage/pkg/configurator"
	"github.com/aws/eks-anywhere-packages/credentialproviderpackage/pkg/configurator/bottlerocket"
	"github.com/aws/eks-anywhere-packages/credentialproviderpackage/pkg/configurator/linux"
	"github.com/aws/eks-anywhere-packages/credentialproviderpackage/pkg/constants"
	"github.com/aws/eks-anywhere-packages/credentialproviderpackage/pkg/log"
)

const (
	bottleRocket = "bottlerocket"
	socketPath   = "/run/api.sock"

	// Aws Credentials
	awsProfile    = "eksa-packages"
	credWatchData = "/secrets/aws-creds/..data"
	credWatchPath = "/secrets/aws-creds/"
)

func main() {
	var configurator cfg.Configurator
	var err error
	osType := strings.ToLower(os.Getenv("OS_TYPE"))
	if osType == "" {
		log.ErrorLogger.Println("Missing Environment Variable OS_TYPE")
		os.Exit(1)
	}
	secretPath, err := awscred.GetAwsConfigPath()
	if err != nil {
		log.ErrorLogger.Fatal(err)
	}
	profile := os.Getenv("AWS_PROFILE")
	if profile == "" {
		profile = awsProfile
	}
	config := createCredentialProviderConfigOptions()
	if osType == bottleRocket {
		configurator, err = bottlerocket.NewBottleRocketConfigurator(socketPath)
		if err != nil {
			log.ErrorLogger.Fatal(err)
		}
	} else {
		configurator = linux.NewLinuxConfigurator()
	}

	configurator.Initialize(config)
	err = configurator.UpdateAWSCredentials(secretPath, profile)
	if err != nil {
		log.ErrorLogger.Fatal(err)
	}
	log.InfoLogger.Println("Aws credentials configured")

	err = configurator.UpdateCredentialProvider(profile)
	if err != nil {
		log.ErrorLogger.Fatal(err)
	}
	log.InfoLogger.Println("Credential Provider Configured")

	err = configurator.CommitChanges()
	if err != nil {
		log.ErrorLogger.Fatal(err)
	}

	log.InfoLogger.Println("Kubelet Restarted")

	// Creating watcher for credentials
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.ErrorLogger.Fatal(err)
	}
	defer watcher.Close()

	// Start listening for changes to the aws credentials
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Create) {
					if event.Name == credWatchData {
						secretPath, err := awscred.GetAwsConfigPath()
						if err != nil {
							log.ErrorLogger.Fatal(err)
						}
						err = configurator.UpdateAWSCredentials(secretPath, profile)
						if err != nil {
							log.ErrorLogger.Fatal(err)
						}
						log.InfoLogger.Println("Aws credentials successfully changed")
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.WarningLogger.Printf("filewatcher error: %v", err)
			}
		}
	}()

	err = watcher.Add(credWatchPath)
	if err != nil {
		log.ErrorLogger.Fatal(err)
	}

	// Block main goroutine forever.
	<-make(chan struct{})
}

func createCredentialProviderConfigOptions() constants.CredentialProviderConfigOptions {
	imagePatternsValues := os.Getenv("MATCH_IMAGES")
	if imagePatternsValues == "" {
		imagePatternsValues = constants.DefaultImagePattern
	}
	imagePatterns := strings.Split(imagePatternsValues, ",")

	defaultCacheDuration := os.Getenv("DEFAULT_CACHE_DURATION")
	if defaultCacheDuration == "" {
		defaultCacheDuration = constants.DefaultCacheDuration
	}

	return constants.CredentialProviderConfigOptions{
		ImagePatterns:        imagePatterns,
		DefaultCacheDuration: defaultCacheDuration,
	}
}
