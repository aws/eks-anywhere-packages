package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// Options represents the flag for the current plugin
type Options struct {
	inputFile      string
	outputFolder   string
	generateSample bool
	promote        string
	key            string
	publicProfile  string
	privateProfile string
	bundleFile     string
	regionCheck    bool
}

func (o *Options) SetupLogger() {
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
}

// Validate validates the receiving options.
func (o *Options) ValidateInput() ([]string, error) {
	f := []string{}
	if o.inputFile != "" {
		f = append(f, o.inputFile)
		return f, nil
	}
	f, err := getYamlFiles()
	if err != nil {
		BundleLog.Error(err, "Error getting yaml files from stdin")
		return nil, err
	}
	return f, nil
}

func getYamlFiles() ([]string, error) {
	f := []string{}
	pwd, _ := os.Getwd()
	err := filepath.Walk(pwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return err
		}
		endsWith := strings.HasSuffix(path, ".yaml")
		if endsWith {
			notoutput := strings.Contains(path, "output/")
			if !notoutput {
				f = append(f, path)
			}
		}
		return nil
	})
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return f, err
}

// NewOptions instantiates Options from arguments
func NewOptions() *Options {
	o := Options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.BoolVar(&o.generateSample, "generate-sample", false, "Whether you want to generate a sample bundle for yourself")
	fs.StringVar(&o.inputFile, "input", "", "The path where the input bundle generation file lives")
	fs.StringVar(&o.outputFolder, "output", "output", "The path where to write the output bundle files")
	fs.StringVar(&o.promote, "promote", "", "The Helm chart private ECR OCI uri to pull and promote")
	fs.StringVar(&o.key, "key", "k", "The key to sign with")
	fs.StringVar(&o.publicProfile, "public-profile", "", "The AWS Public Profile to release the prod bundle into")
	fs.StringVar(&o.privateProfile, "private-profile", "", "The AWS Private Profile to release all packages into")
	fs.StringVar(&o.bundleFile, "bundle", "", "The path where the bundle file lives")
	fs.BoolVar(&o.regionCheck, "region-check", false, "Check the passed in bundle resource and if they exist in the supported ECR regions")
	err := fs.Parse(os.Args[1:])
	if err != nil {
		BundleLog.Error(err, "Error parsing input flags")
	}
	return &o
}
