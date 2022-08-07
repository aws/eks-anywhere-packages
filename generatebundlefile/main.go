package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
	"gopkg.in/yaml.v2"
	ctrl "sigs.k8s.io/controller-runtime"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	sig "github.com/aws/eks-anywhere-packages/pkg/signature"
)

var BundleLog = ctrl.Log.WithName("BundleGenerator")

func main() {
	o := NewOptions()
	o.SetupLogger()

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
		BundleLog.Info("Starting Promote from private ECR to Public ECR....")
		clients, err := GetSDKClients()
		if err != nil {
			BundleLog.Error(err, "getting SDK clients")
			os.Exit(1)
		}
		clients.ecrPublicClient.SourceRegistry, err = clients.ecrPublicClient.GetRegistryURI()
		if err != nil {
			BundleLog.Error(err, "getting registry URI")
			os.Exit(1)
		}
		dockerStruct := &DockerAuth{
			Auths: map[string]DockerAuthRegistry{
				fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", clients.stsClient.AccountID, ecrRegion): {clients.ecrClient.AuthConfig},
				"public.ecr.aws": {clients.ecrPublicClient.AuthConfig},
			},
		}
		dockerAuth, err := NewAuthFile(dockerStruct)
		if err != nil || dockerAuth.Authfile == "" {
			BundleLog.Error(err, "Unable create AuthFile")
			os.Exit(1)
		}
		err = clients.PromoteHelmChart(o.promote, dockerAuth.Authfile, false)
		if err != nil {
			BundleLog.Error(err, "Unable to promote Helm Chart")
			os.Exit(1)
		}
		err = dockerAuth.Remove()
		if err != nil {
			BundleLog.Error(err, "Unable to remove docker auth file")
			os.Exit(1)
		}

		BundleLog.Info("Promote Finished, exiting gracefully")
		return
	}

	// validate that an input flag is either given, or the system can find yaml files to use.
	files, err := o.ValidateInput()
	if err != nil {
		BundleLog.Error(err, "Unable to validate input flag, or find local yaml files for input")
		os.Exit(1)
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
		BundleLog.Error(err, "creating public ECR client")
		os.Exit(1)
	}
	conf, err = config.LoadDefaultConfig(context.TODO(), config.WithRegion(ecrRegion))
	if err != nil {
		BundleLog.Error(err, "loading default AWS config: %w", err)
		os.Exit(1)
	}
	ecrClient := ecr.NewFromConfig(conf)
	clients.ecrClient, err = NewECRClient(ecrClient, true)
	if err != nil {
		BundleLog.Error(err, "creating ECR client")
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
				fmt.Sprintf("public.ecr.aws/%s", clients.ecrPublicClient.SourceRegistry): {clients.ecrPublicClient.AuthConfig},
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
			charts.Source.Versions[0].Schema = helmRequires.Spec.Schema

			// Set the registry to empty string since we pull it from the PackageBundleController instead now.
			addOnBundleSpec.Packages[i].Source.Registry = ""
		}

		err = dockerAuth.Remove()
		if err != nil {
			BundleLog.Error(err, "unable to remove docker auth file")
			os.Exit(1)
		}

		// Write list of bundle structs into Bundle CRD files
		BundleLog.Info("In Progress: Writing bundle to output")
		bundle := AddMetadata(addOnBundleSpec, name)

		// We will make a compound check for public and private profile after the launch once we want to stop
		// push packages to private ECR.
		if o.publicProfile != "" {
			BundleLog.Info("Starting release public ECR process....")
			clients, err := GetSDKClients()
			if err != nil {
				BundleLog.Error(err, "getting sdk clients")
				os.Exit(1)
			}

			clients, err = clients.GetProfileSDKConnection("ecrpublic", o.publicProfile)
			if err != nil {
				BundleLog.Error(err, "Unable create SDK Client connections")
				os.Exit(1)
			}

			clients.ecrPublicClient.SourceRegistry, err = clients.ecrPublicClient.GetRegistryURI()
			if err != nil {
				BundleLog.Error(err, "Unable create find Public ECR URI from calling account")
				os.Exit(1)
			}
			clients.ecrPublicClientRelease.SourceRegistry, err = clients.ecrPublicClientRelease.GetRegistryURI()
			if err != nil {
				BundleLog.Error(err, "Unable create find Public ECR URI for destination account")
				os.Exit(1)
			}
			dockerReleaseStruct = &DockerAuth{
				Auths: map[string]DockerAuthRegistry{
					fmt.Sprintf("public.ecr.aws/%s", clients.ecrPublicClient.SourceRegistry): {clients.ecrPublicClient.AuthConfig},
					"public.ecr.aws": {clients.ecrPublicClientRelease.AuthConfig},
				},
			}
			dockerAuth, err = NewAuthFile(dockerReleaseStruct)
			if err != nil || dockerAuth.Authfile == "" {
				BundleLog.Error(err, "Unable create AuthFile")
				os.Exit(1)
			}
			for _, charts := range addOnBundleSpec.Packages {
				err = clients.PromoteHelmChart(charts.Source.Repository, dockerAuth.Authfile, true)
				if err != nil {
					BundleLog.Error(err, "promoting helm chart",
						"name", charts.Source.Repository)
					os.Exit(1)
				}
			}
			err = dockerAuth.Remove()
			if err != nil {
				BundleLog.Error(err, "removing docker auth file")
				os.Exit(1)
			}

			return
		}

		// See above comment about compound check when we want to cutover
		// if o.publicProfile != "" && if o.privateProfile != "" {}
		if o.privateProfile != "" {
			BundleLog.Info("Starting release to private ECR process....")
			clients, err := GetSDKClients()
			if err != nil {
				BundleLog.Error(err, "getting SDK clients")
				os.Exit(1)
			}

			clients, err = clients.GetProfileSDKConnection("ecr", o.privateProfile)
			if err != nil {
				BundleLog.Error(err, "getting SDK connection")
				os.Exit(1)
			}
			clients, err = clients.GetProfileSDKConnection("sts", o.privateProfile)
			if err != nil {
				BundleLog.Error(err, "getting profile SDK connection")
				os.Exit(1)
			}
			dockerReleaseStruct = &DockerAuth{
				Auths: map[string]DockerAuthRegistry{
					fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", clients.stsClient.AccountID, ecrRegion):        {clients.ecrClient.AuthConfig},
					fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", clients.stsClientRelease.AccountID, ecrRegion): {clients.ecrClientRelease.AuthConfig},
				},
			}
			dockerAuth, err = NewAuthFile(dockerReleaseStruct)
			if err != nil {
				BundleLog.Error(err, "Unable create AuthFile")
				os.Exit(1)
			}
			for _, charts := range addOnBundleSpec.Packages {
				err = clients.PromoteHelmChart(charts.Source.Repository, dockerAuth.Authfile, false)
				if err != nil {
					BundleLog.Error(err, "promoting helm chart",
						"name", charts.Source.Repository)
					os.Exit(1)
				}

			}
			err = dockerAuth.Remove()
			if err != nil {
				BundleLog.Error(err, "removing docker auth file")
				os.Exit(1)
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
