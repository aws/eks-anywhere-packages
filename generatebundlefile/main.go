package main

import (
	"fmt"
	"os"
	"path/filepath"

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
		err := WriteBundleConfig(sample.Spec, outputPath)
		if err != nil {
			BundleLog.Error(err, "Unable to create CRD skaffolding from generatesample command")
		}
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

	for _, f := range files {
		Inputs, err := ValidateInputConfig(f)
		if err != nil {
			BundleLog.Error(err, "Unable to validate input file")
			os.Exit(1)
		}
		BundleLog.Info("In Progress: Populating Bundles and looking up Sha256 tags")
		crd, err := Inputs.NewBundleFromInput()
		if err != nil {
			BundleLog.Error(err, "Unable to create CRD skaffolding of AddoOBundle from input file")
			os.Exit(1)
		}
		// Write list of bundle structs into Bundle CRD files
		BundleLog.Info("In Progress: Writing output files")
		err = WriteBundleConfig(crd, outputPath)
		if err != nil {
			BundleLog.Error(err, "Unable to write bundleconfig from Bundle struct")
			os.Exit(1)
		}
		BundleLog.Info("Finished writing output crd files.", "Output path", fmt.Sprintf("%s%s", o.outputFolder, "/"))
	}
}
