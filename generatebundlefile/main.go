package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	sig "github.com/aws/eks-anywhere-packages/pkg/signature"
	"gopkg.in/yaml.v2"
	ctrl "sigs.k8s.io/controller-runtime"
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

	for _, f := range files {
		Inputs, err := ValidateInputConfig(f)
		if err != nil {
			BundleLog.Error(err, "Unable to validate input file")
			os.Exit(1)
		}
		BundleLog.Info("In Progress: Populating Bundles and looking up Sha256 tags")
		addOnBundleSpec, name, err := Inputs.NewBundleFromInput()
		if err != nil {
			BundleLog.Error(err, "Unable to create CRD skaffolding of AddoOBundle from input file")
			os.Exit(1)
		}
		// Write list of bundle structs into Bundle CRD files
		BundleLog.Info("In Progress: Writing bundle to output")
		bundle := AddMetadata(addOnBundleSpec, name)

		// // If we trigger a release,
		if o.release != "" {
			BundleLog.Info("Starting release process....")
			clients, err := GetSDKClients(o.release)
			clients.ecrPublicClient.SourceRegistry, err = clients.ecrPublicClient.GetRegistryURI()
			if err != nil {
				BundleLog.Error(err, "Unable lookup ECRPublic Registry URI")
				os.Exit(1)
			}
			clients.ecrPublicClientRelease.SourceRegistry, err = clients.ecrPublicClientRelease.GetRegistryURI()
			if err != nil {
				BundleLog.Error(err, "Unable lookup ECRPublic Registry URI for target account")
				os.Exit(1)
			}
			dockerReleaseStruct := &DockerAuth{
				Auths: map[string]DockerAuthRegistry{
					fmt.Sprintf("public.ecr.aws/%s", clients.ecrPublicClient.SourceRegistry): DockerAuthRegistry{clients.ecrPublicClient.AuthConfig},
					"public.ecr.aws": DockerAuthRegistry{clients.ecrPublicClientRelease.AuthConfig},
				},
			}
			authFile, err := NewAuthFile(dockerReleaseStruct)
			if err != nil || authFile == "" {
				BundleLog.Error(err, "Unable create AuthFile")
				os.Exit(1)
			}
			defer os.Remove(authFile)
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
