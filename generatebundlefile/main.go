package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	sig "github.com/aws/eks-anywhere-packages/pkg/signature"
)

var BundleLog = ctrl.Log.WithName("BundleGenerator")

func main() {
	o := NewOptions()

	// grab local path to caller, and make new caller
	pwd, err := os.Getwd()
	if err != nil {
		BundleLog.Error(err, "Unable to get current working directory")
		os.Exit(1)
	}
	outputDir := filepath.Join(pwd, o.outputFolder)
	outputPath, err := NewWriter(outputDir)
	if err != nil {
		BundleLog.Error(err, "Unable to create new Writer")
		os.Exit(1)
	}

	// If using --generatesample flag we skip the yaml input portion
	if o.generateSample {
		sample := NewBundleGenerate("generatesample")

		_, yml, err := sig.GetDigest(sample, sig.EksaDomain)
		if err != nil {
			BundleLog.Error(err, "Unable to convert Bundle to yaml via sig.GetDigest()")
			os.Exit(1)
		}
		if _, err := outputPath.Write("bundle.yaml", yml, PersistentFile); err != nil {
			BundleLog.Error(err, "Unable to write Bundle to yaml from generateSample")
			os.Exit(1)
		}
		return
	}

	if o.promote != "" {
		fmt.Printf("Promoting %s from private ECR to Public ECR\n", o.promote)
		clients, err := GetSDKClients("")
		clients.ecrPublicClient.SourceRegistry, err = clients.ecrPublicClient.GetRegistryURI()
		dockerStruct := &DockerAuth{
			Auths: map[string]DockerAuthRegistry{
				fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", clients.stsClient.AccountID, ecrRegion): DockerAuthRegistry{clients.ecrClient.AuthConfig},
				"public.ecr.aws": DockerAuthRegistry{clients.ecrPublicClient.AuthConfig},
			},
		}
		authFile, err := NewAuthFile(dockerStruct)
		if err != nil || authFile == "" {
			BundleLog.Error(err, "Unable create AuthFile")
		}
		defer os.Remove(authFile)
		err = clients.PromoteHelmChart(o.promote, authFile, false)
		if err != nil {
			BundleLog.Error(err, "Unable to promote Helm Chart")
		}
		fmt.Printf("Promote Finished, exiting gracefully\n")
		return
	}

	// validate that an input flag is either given, or the system can find yaml files to use.
	files, err := o.ValidateInput()
	if err != nil {
		BundleLog.Error(err, "Unable to validate input flag, or find local yaml files for input")
		os.Exit(1)
	}

	// One input file, and a --signature input
	if o.signature != "" && len(files) == 1 {
		BundleLog.Info("In Progress: Checking Bundles for Signatures")
		bundle, err := ValidateBundle(files[0])
		if err != nil {
			BundleLog.Error(err, "Unable to validate bundle file")
			os.Exit(1)
		}
		// Check if there is annotations on the bundle
		// If no annotations, add them
		// If annotations, check for signatures and compare with input
		check, err := IfSignature(bundle)
		if !check {
			BundleLog.Info("Adding Signature to bundle and exiting...")
			bundle.ObjectMeta.Annotations[FullSignatureAnnotation] = o.signature
		} else {
			// If annotations do currently exist then compare the current signature vs the input signature
			BundleLog.Info("Signature already exists on bundle checking it's contents...")
			check, err := CheckSignature(bundle, o.signature)
			if err != nil || !check {
				BundleLog.Error(err, "Unable to compare signatures")
				os.Exit(1)
			}
		}
		_, yml, err := sig.GetDigest(bundle, sig.EksaDomain)
		if err != nil {
			BundleLog.Error(err, "Unable to convert Bundle to yaml via sig.GetDigest()")
			os.Exit(1)
		}
		if _, err := outputPath.Write("bundle.yaml", yml, PersistentFile); err != nil {
			BundleLog.Error(err, "Unable to write Bundle to yaml from signature flag")
			os.Exit(1)
		}
		return
	}

	// Validate Input config, and turn into Input struct
	BundleLog.Info("Using input file to create bundle crds.", "Input file", o.inputFile)

	conf, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(ecrPublicRegion))
	if err != nil {
		BundleLog.Error(err, "loading default AWS config: %w", err)
		os.Exit(1)
	}
	clients := &SDKClients{}
	client := ecrpublic.NewFromConfig(conf)
	clients.ecrPublicClient, err = NewECRPublicClient(client, true)
	if err != nil {
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
				fmt.Sprintf("public.ecr.aws/%s", clients.ecrPublicClient.SourceRegistry): DockerAuthRegistry{clients.ecrPublicClient.AuthConfig},
			},
		}
		authFile, err := NewAuthFile(dockerReleaseStruct)
		if err != nil || authFile == "" {
			BundleLog.Error(err, "Unable create AuthFile")
		}
		defer os.Remove(authFile)
		driver, err := NewHelm(BundleLog, authFile)
		if err != nil {
			BundleLog.Error(err, "Unable to create Helm driver")
			os.Exit(1)
		}

		BundleLog.Info("In Progress: Populating Bundles and looking up Sha256 tags")
		addOnBundleSpec, name, err := clients.ecrPublicClient.NewBundleFromInput(Inputs)
		if err != nil {
			BundleLog.Error(err, "Unable to create CRD skaffolding of AddoOBundle from input file")
			os.Exit(1)
		}

		// Pull Helm charts for all the populated helm fields of the bundles.
		for _, charts := range addOnBundleSpec.Packages {
			fullURI := fmt.Sprintf("%s/%s", charts.Source.Registry, charts.Source.Repository)
			chartPath, err := driver.PullHelmChart(fullURI, charts.Source.Versions[0].Name)
			if err != nil {
				BundleLog.Error(err, "Unable to pull Helm Chart %s", charts.Source.Repository)
				os.Exit(1)
			}
			chartName, helmname, err := splitECRName(fullURI)
			if err != nil {
				BundleLog.Error(err, "Unable to split helm hame, invalid format")
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
			helmConfiguration := []api.VersionConfiguration{}
			for _, config := range helmRequires.Spec.Configurations {
				helmConfiguration = append(helmConfiguration, api.VersionConfiguration{
					Name: config.Name,
				})
			}
			charts.Source.Versions[0].Configurations = helmConfiguration
		}

		// Write list of bundle structs into Bundle CRD files
		BundleLog.Info("In Progress: Writing bundle to output")
		bundle := AddMetadata(addOnBundleSpec, name)

		// // If we trigger a release,
		if o.release != "" {
			BundleLog.Info("Starting release process....")
			clients, err := GetSDKClients(o.release)
			if err != nil {
				BundleLog.Error(err, "Unable create SDK Client connections")
			}
			clients.ecrPublicClient.SourceRegistry, err = clients.ecrPublicClient.GetRegistryURI()
			if err != nil {
				BundleLog.Error(err, "Unable create find Public ECR URI from calling account")
			}
			clients.ecrPublicClientRelease.SourceRegistry, err = clients.ecrPublicClientRelease.GetRegistryURI()
			if err != nil {
				BundleLog.Error(err, "Unable create find Public ECR URI for destination account")
			}
			dockerReleaseStruct = &DockerAuth{
				Auths: map[string]DockerAuthRegistry{
					fmt.Sprintf("public.ecr.aws/%s", clients.ecrPublicClient.SourceRegistry): DockerAuthRegistry{clients.ecrPublicClient.AuthConfig},
					"public.ecr.aws": DockerAuthRegistry{clients.ecrPublicClientRelease.AuthConfig},
				},
			}
			authFile, err = NewAuthFile(dockerReleaseStruct)
			if err != nil || authFile == "" {
				BundleLog.Error(err, "Unable create AuthFile")
			}
			for _, charts := range addOnBundleSpec.Packages {
				err = clients.PromoteHelmChart(charts.Source.Repository, authFile, true)
			}
			return
		}

		signature, err := GetBundleSignature(context.Background(), bundle, o.key)
		if err != nil {
			BundleLog.Error(err, "Unable to sign bundle with kms key")
			os.Exit(1)
		}

		//Remove excludes before generating YAML so that registry + repository remains
		bundle.ObjectMeta.Annotations[FullExcludesAnnotation] = ""
		_, yml, err := sig.GetDigest(bundle, sig.EksaDomain)
		if err != nil {
			BundleLog.Error(err, "Unable to retrieve and generate Digest from manifest")
			os.Exit(1)
		}
		manifest := make(map[interface{}]interface{})
		err = yaml.Unmarshal(yml, &manifest)
		if err != nil {
			BundleLog.Error(err, "Unable to marshal manifest into yaml bytes")
			os.Exit(1)
		}
		anno := manifest["metadata"].(map[interface{}]interface{})["annotations"].(map[interface{}]interface{})
		anno[FullSignatureAnnotation] = signature
		anno[FullExcludesAnnotation] = Excludes
		yml, err = yaml.Marshal(manifest)
		if _, err := outputPath.Write("bundle.yaml", yml, PersistentFile); err != nil {
			BundleLog.Error(err, "Unable to write Bundle to yaml")
			os.Exit(1)
		}
		BundleLog.Info("Finished writing output crd files.", "Output path", fmt.Sprintf("%s%s", o.outputFolder, "/"))
	}
}
