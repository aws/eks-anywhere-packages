package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ctrl "sigs.k8s.io/controller-runtime"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	sig "github.com/aws/eks-anywhere-packages/pkg/signature"
)

const (
	defaultRegion = "us-west-2"
)

var BundleLog = ctrl.Log.WithName("BundleGenerator")

func main() {
	opts := NewOptions()
	opts.SetupLogger()

	if opts.generateSample {
		outputFilename := filepath.Join(opts.outputFolder, "bundle.yaml")
		f, err := os.OpenFile(outputFilename, os.O_WRONLY|os.O_CREATE, 0o644)
		if err != nil {
			BundleLog.Error(err, fmt.Sprintf("opening output file %q", outputFilename))
			os.Exit(1)
		}
		defer f.Close()

		err = cmdGenerateSample(f)
		if err != nil {
			BundleLog.Error(err, "generating sample bundle")
			os.Exit(1)
		}

		fmt.Printf("sample bundle file written to %q\n", outputFilename)
		return
	}

	err := cmdGenerate(opts)
	if err != nil {
		BundleLog.Error(err, "generating bundle")
		os.Exit(1)
	}
}

// cmdGenerateSample writes a sample bundle file to the given output folder.
func cmdGenerateSample(w io.Writer) error {
	sample := NewBundleGenerate("generatesample")
	_, yml, err := sig.GetDigest(sample, sig.EksaDomain)
	if err != nil {
		return fmt.Errorf("generating bundle digest: %w", err)
	}

	_, err = w.Write(yml)
	if err != nil {
		return fmt.Errorf("writing sample bundle data: %w", err)
	}

	return nil
}

func cmdGenerate(opts *Options) error {
	// grab local path to caller, and make new caller
	pwd, err := os.Getwd()
	if err != nil {
		BundleLog.Error(err, "Unable to get current working directory")
		os.Exit(1)
	}

	// validate that an input flag is either given, or the system can find yaml files to use.
	files, err := opts.ValidateInput()
	if err != nil {
		return err
	}

	// Validate Input config, and turn into Input struct
	BundleLog.Info("Using input file to create bundle crds.", "Input file", opts.inputFile)
	conf, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(defaultRegion))
	if err != nil {
		BundleLog.Error(err, "loading default AWS config: %w", err)
		os.Exit(1)
	}
	clients := &SDKClients{}
	ecrClient := ecr.NewFromConfig(conf)
	clients.ecrClient, err = NewECRClient(ecrClient, true)
	if err != nil {
		BundleLog.Error(err, "creating ECR client")
		os.Exit(1)
	}

	stsClient := sts.NewFromConfig(conf)
	clients.stsClient, err = NewStsClient(stsClient, true)
	if err != nil {
		BundleLog.Error(err, "creating STS client")
		os.Exit(1)
	}
	for _, f := range files {
		Inputs, err := ValidateInputConfig(f)
		if err != nil {
			BundleLog.Error(err, "Unable to validate input file")
			os.Exit(1)
		}

		// Create Authfile for Helm Driver
		dockerReleaseStruct := &DockerAuth{
			Auths: map[string]DockerAuthRegistry{
				fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", clients.stsClient.AccountID, defaultRegion): {
					clients.ecrClient.AuthConfig,
				},
			},
		}

		dockerAuth, err := NewAuthFile(dockerReleaseStruct)
		if err != nil || dockerAuth.Authfile == "" {
			BundleLog.Error(err, "Unable create AuthFile")
			os.Exit(1)
		}
		driver, err := NewHelm(BundleLog, dockerAuth.Authfile)
		if err != nil {
			BundleLog.Error(err, "Unable to create Helm driver")
			os.Exit(1)
		}

		BundleLog.Info("In Progress: Populating Bundles and looking up Sha256 tags")
		addOnBundleSpec, name, err := clients.NewBundleFromInput(Inputs)
		if err != nil {
			BundleLog.Error(err, "Unable to create bundle from input file")
			os.Exit(1)
		}

		// Pull Helm charts for all the populated helm fields of the bundles.
		for i, charts := range addOnBundleSpec.Packages {
			fullURI := fmt.Sprintf("%s/%s", charts.Source.Registry, charts.Source.Repository)
			chartPath, err := driver.PullHelmChart(fullURI, charts.Source.Versions[0].Name)
			if err != nil {
				BundleLog.Error(err, "Unable to pull Helm Chart")
				os.Exit(1)
			}
			chartName, helmname, err := splitECRName(fullURI)
			if err != nil {
				BundleLog.Error(err, "Unable to split helm name, invalid format")
				os.Exit(1)
			}
			dest := filepath.Join(pwd, chartName)
			err = UnTarHelmChart(chartPath, chartName, dest)
			if err != nil {
				BundleLog.Error(err, "Unable to untar Helm Chart")
				os.Exit(1)
			}
			// Check for requires.yaml in the unpacked helm chart
			helmDest := filepath.Join(pwd, chartName, helmname)
			defer os.RemoveAll(helmDest)
			f, err := hasRequires(helmDest)
			if err != nil {
				BundleLog.Error(err, "Helm chart doesn't have requires.yaml inside")
				os.Exit(1)
			}
			// Unpack requires.yaml into a GO struct
			helmRequires, err := validateHelmRequires(f)
			if err != nil {
				BundleLog.Error(err, "Unable to parse requires.yaml file to Go Struct")
				os.Exit(1)
			}
			// Populate Images to bundle spec from Requires.yaml
			helmImage := []api.VersionImages{}
			for _, image := range helmRequires.Spec.Images {
				helmImage = append(helmImage, api.VersionImages{
					Repository: image.Repository,
					Digest:     image.Digest,
				})
			}
			charts.Source.Versions[0].Images = helmImage
			// Populate Configurations to bundle spec from Requires.yaml

			if len(deleteEmptyStringSlice(helmRequires.Spec.Dependencies)) > 0 {
				charts.Source.Versions[0].Dependencies = helmRequires.Spec.Dependencies
			}
			charts.Source.Versions[0].Schema = helmRequires.Spec.Schema

			// Set the registry to empty string since we pull it from the PackageBundleController instead now.
			addOnBundleSpec.Packages[i].Source.Registry = ""
		}

		err = dockerAuth.Remove()
		if err != nil {
			BundleLog.Error(err, "unable to remove docker auth file")
			os.Exit(1)
		}
		bundle := AddMetadata(addOnBundleSpec, name)

		bundle.Annotations[FullExcludesAnnotation] = Excludes
		BundleLog.Info("Generating bundle signature", "key", opts.key)
		signature, err := GetBundleSignature(context.Background(), bundle, opts.key)
		if err != nil {
			BundleLog.Error(err, "Unable to sign bundle with kms key")
			os.Exit(1)
		}
		bundle.Annotations[FullSignatureAnnotation] = signature

		yml, err := serializeBundle(bundle)
		if err != nil {
			BundleLog.Error(err, "marshaling bundle YAML: %w", err)
			os.Exit(1)
		}

		BundleLog.Info("In Progress: Writing bundle to output")
		outputDir := filepath.Join(pwd, opts.outputFolder)
		outputPath, err := NewWriter(outputDir)
		if err != nil {
			BundleLog.Error(err, "Unable to create new Writer")
			os.Exit(1)
		}

		if _, err := outputPath.Write("bundle.yaml", yml, PersistentFile); err != nil {
			BundleLog.Error(err, "Unable to write Bundle to yaml")
			os.Exit(1)
		}
		BundleLog.Info("Finished writing output crd files.", "Output path", fmt.Sprintf("%s%s", opts.outputFolder, "/"))
	}

	return nil
}
