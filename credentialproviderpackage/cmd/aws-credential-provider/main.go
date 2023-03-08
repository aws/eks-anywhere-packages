package main

import (
	_ "embed"
	"io/fs"
	"os"
	"strings"

	"github.com/fsnotify/fsnotify"

	cfg "credential-provider/pkg/configurator"
	"credential-provider/pkg/configurator/bottlerocket"
	"credential-provider/pkg/configurator/linux"
	"credential-provider/pkg/constants"
	"credential-provider/pkg/log"
)

func main() {
	var configurator cfg.Configurator
	osType := strings.ToLower(os.Getenv("OS_TYPE"))
	if osType == "" {
		log.ErrorLogger.Println("Missing Environment Variable OS_TYPE")
		os.Exit(1)
	}
	profile := os.Getenv("AWS_PROFILE")
	if profile == "" {
		profile = constants.Profile
	}
	config := createCredentialProviderConfigOptions()
	if osType == constants.BottleRocket {
		socket, err := os.Stat(constants.SocketPath)
		if err != nil {
			log.ErrorLogger.Fatal(err)
		}
		if socket.Mode().Type() == fs.ModeSocket {
			configurator = bottlerocket.NewBottleRocketConfigurator(constants.SocketPath)

		} else {
			log.ErrorLogger.Fatalf("Unexpected type %s expected socket\n", socket.Mode().Type())
		}
	} else {
		configurator = linux.NewLinuxConfigurator()
	}

	configurator.Initialize(config)
	err := configurator.UpdateAWSCredentials(constants.CredSrcPath, profile)
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
					if event.Name == constants.CredWatchData {
						err = configurator.UpdateAWSCredentials(constants.CredSrcPath, profile)
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

	err = watcher.Add(constants.CredWatchPath)
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
