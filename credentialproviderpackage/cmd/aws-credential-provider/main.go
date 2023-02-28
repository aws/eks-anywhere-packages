package main

import (
	_ "embed"
	"io/fs"
	"log"
	"os"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	cfg "credential-provider/pkg/configurator"
	"credential-provider/pkg/configurator/bottlerocket"
	"credential-provider/pkg/configurator/linux"
	"credential-provider/pkg/constants"
	"credential-provider/pkg/utils"
)

func checkErrAndLog(err error, logger *log.Logger) {
	if err != nil {
		logger.Println(err)
		os.Exit(1)
	}
}

func main() {
	utils.InfoLogger.Println("Running at " + time.Now().UTC().String())

	var configurator cfg.Configurator
	osType := strings.ToLower(os.Getenv("OS_TYPE"))
	if osType == "" {
		utils.ErrorLogger.Println("Missing Environment Variable OS")
		os.Exit(1)
	}
	profile := os.Getenv("AWS_PROFILE")
	if profile == "" {
		profile = constants.Profile
	}
	config := createCredentialProviderConfigOptions()
	if osType == constants.BottleRocket {
		socket, err := os.Stat(constants.SocketPath)
		checkErrAndLog(err, utils.ErrorLogger)
		if socket.Mode().Type() == fs.ModeSocket {
			configurator = bottlerocket.NewBottleRocketConfigurator(constants.SocketPath)

		} else {
			utils.ErrorLogger.Printf("Unexpected type %s expected socket\n", socket.Mode().Type())
			os.Exit(1)
		}
	} else {
		configurator = linux.NewLinuxConfigurator()
	}

	configurator.Initialize(config)
	err := configurator.UpdateAWSCredentials(constants.CredSrcPath, profile)
	checkErrAndLog(err, utils.ErrorLogger)
	utils.InfoLogger.Println("Aws credentials configured")

	err = configurator.UpdateCredentialProvider(profile)
	checkErrAndLog(err, utils.ErrorLogger)
	utils.InfoLogger.Println("Credential Provider Configured")

	err = configurator.CommitChanges()
	checkErrAndLog(err, utils.ErrorLogger)

	utils.InfoLogger.Println("Kubelet Restarted")

	// Creating watcher for credentials
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
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
						checkErrAndLog(err, utils.ErrorLogger)
						utils.InfoLogger.Println("Aws credentials successfully changed")
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(constants.CredWatchPath)
	if err != nil {
		log.Fatal(err)
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
