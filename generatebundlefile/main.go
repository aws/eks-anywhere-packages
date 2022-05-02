package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
		err := WriteBundleConfig(sample, outputPath)
		if err != nil {
			BundleLog.Error(err, "Unable to create CRD skaffolding from generatesample command")
		}
		return
	}

	if o.promote != "" {
		fmt.Printf("Promoting %s from private ECR to Public ECR\n", o.promote)
		// Get AWS Clients
		ecrPublicClient, err := NewECRPublicClient(true)
		if err != nil {
			BundleLog.Error(err, "Unable to create SDK connection to ECR Public")
		}
		ecrClient, err := NewECRClient(true)
		if err != nil {
			BundleLog.Error(err, "Unable to create SDK connection to ECR")
		}
		stsClient, err := NewStsClient(true)
		if err != nil {
			BundleLog.Error(err, "Unable to create SDK connection to STS")
		}
		authFile, err := NewAuthFile(ecrClient.AuthConfig, ecrPublicClient.AuthConfig, stsClient.AccountID)
		if err != nil || authFile == "" {
			BundleLog.Error(err, "Unable create AuthFile")
		}
		defer os.Remove(authFile)

		name, version, err := ecrClient.getNameAndVersion(o.promote, stsClient.AccountID)
		fmt.Printf("Promoting chart and image version %s:%s\n", name, version)
		semVer := strings.Replace(version, "_", "+", 1) // TODO use the Semvar library instead of this hack.

		// Pull the Helm chart to Helm Cache
		chartPath, err := PullHelmChart(name, semVer)
		if err != nil {
			BundleLog.Error(err, "Failed pulling Helm Chart")
		}
		// Get the correct Repo Name from the flag based on ECR repo name formatting
		// since we prepend the github org on some repo's, and not on others.
		chartName, helmname, err := splitECRName(name)
		if err != nil {
			BundleLog.Error(err, "Failed splitECRName")
		}
		// Untar the helm .tgz to pwd and name the folder to the helm chart Name
		dest := filepath.Join(pwd, chartName)
		err = UnTarHelmChart(chartPath, chartName, dest)
		if err != nil {
			BundleLog.Error(err, "failed pulling helm release %s", name)
		}

		// Check for requires.yaml in the unpacked helm chart
		helmDest := filepath.Join(pwd, chartName, helmname)
		f, err := hasRequires(helmDest)
		if err != nil {
			BundleLog.Error(err, "Helm chart doesn't have requires.yaml inside")
		}

		// Unpack requires.yaml into a GO struct
		helmRequires, err := validateHelmRequires(f)
		if err != nil {
			BundleLog.Error(err, "Unable to parse requires.yaml file to Go Struct")
		}

		// Add the helm chart to the struct before looping through lookup/promote since we need it promoted too.
		ecrPublicClient.SourceRegistry, err = ecrPublicClient.GetRegistryURI()
		fmt.Printf("Got ECR Public destination: %s\n", ecrPublicClient.SourceRegistry)
		helmRequires.Spec.Images = append(helmRequires.Spec.Images, Image{Repository: chartName, Digest: version})

		// Loop through each image, and the helm chart itself and check for existance in ECR Public, skip if we find the SHA already exists in destination.
		// If we don't find the SHA in public, we lookup the tag from Private, and copy from private to Public with the same tag.
		for _, images := range helmRequires.Spec.Images {
			check, err := ecrPublicClient.shaExistsInRepository(images.Repository, images.Digest)
			if err != nil {
				BundleLog.Error(err, "Unable to complete sha lookup this is due to an ECRPublic DescribeImages failure")
			}
			if check {
				continue
			} else {
				// If it's a Digest, we lookup the tag, and override it so we have a destination tag in the public Repo.
				if strings.HasPrefix(images.Digest, "sha256") {
					images.Digest, err = ecrClient.tagFromSha(images.Repository, images.Digest)
					if err != nil {
						BundleLog.Error(err, "Unable to find Tag from Digest")
					}
				}
				err := copyImagePrivPubSameAcct(BundleLog, authFile, stsClient, ecrPublicClient, images)
				if err != nil {
					BundleLog.Error(err, "Unable to copy image from source to destination repo")
				}
			}
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
			bundle, err = AddSignature(bundle, o.signature)
		} else {
			// If annotations do currently exist then compare the current signature vs the input signature
			BundleLog.Info("Signature already exists on bundle checking it's contents...")
			check, err := CheckSignature(bundle, o.signature)
			if err != nil || !check {
				BundleLog.Error(err, "Unable to compare signatures")
				os.Exit(1)
			}
		}
		err = WriteBundleConfig(bundle, outputPath)
		if err != nil {
			BundleLog.Error(err, "Unable to write Bundle")
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
		BundleLog.Info("In Progress: Writing output files")
		bundle := AddMetadata(addOnBundleSpec, name)
		err = WriteBundleConfig(bundle, outputPath)
		if err != nil {
			BundleLog.Error(err, "Unable to write Bundle")
			os.Exit(1)
		}
		BundleLog.Info("Finished writing output crd files.", "Output path", fmt.Sprintf("%s%s", o.outputFolder, "/"))
	}
}
