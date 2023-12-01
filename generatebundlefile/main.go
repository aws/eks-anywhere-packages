package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
	ctrl "sigs.k8s.io/controller-runtime"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	sig "github.com/aws/eks-anywhere-packages/pkg/signature"
)

var BundleLog = ctrl.Log.WithName("BundleGenerator")

var Profile = "default"

func main() {
	opts := NewOptions()
	opts.SetupLogger()

	regionalBuildModeEnvvar := os.Getenv("REGIONAL_BUILD_MODE")
	if regionalBuildModeEnvvar == "true" {
		opts.regionalBuildMode = true
	} else {
		opts.regionalBuildMode = false
	}

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

	// Run promotion operation if promote flag is provided or if running
	// in regional build mode
	if opts.promote != "" {
		err := cmdPromote(opts)
		if err != nil {
			BundleLog.Error(err, "promoting curated package")
			os.Exit(1)
		}
		return
	}

	if opts.regionCheck {
		err := cmdRegion(opts)
		if err != nil {
			BundleLog.Error(err, "checking bundle across region")
			os.Exit(1)
		}
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

func cmdPromote(opts *Options) error {
	BundleLog.Info("Starting Promote from private ECR to Public ECR....")

	promoteCharts := make(map[string][]string)

	if opts.tag == "" {
		opts.tag = "latest"
	}

	// If we are promoting an individual chart with the --promote flag like we do for most charts.
	if opts.promote != "" {
		promoteCharts[opts.promote] = append(promoteCharts[opts.promote], opts.tag)
	}

	// If we are promoting multiple chart with the --input file flag we override the struct with files inputs from the file.
	if opts.inputFile != "" {
		packages, err := opts.ValidateInput()
		if err != nil {
			return err
		}
		for _, f := range packages {
			Inputs, err := ValidateInputConfig(f)
			if err != nil {
				BundleLog.Error(err, "Unable to validate input file")
				os.Exit(1)
			}
			delete(promoteCharts, opts.promote) // Clear the promote map to pull values only from file
			for _, p := range Inputs.Packages {
				for _, project := range p.Projects {
					promoteCharts[project.Repository] = append(promoteCharts[project.Repository], project.Versions[0].Name)
				}
			}
		}
	}

	clients, err := GetSDKClients(opts.regionalBuildMode)
	if err != nil {
		return fmt.Errorf("getting SDK clients: %w", err)
	}

	dockerStruct := &DockerAuth{
		Auths: map[string]DockerAuthRegistry{
			fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", clients.stsClient.AccountID, ecrRegion): {clients.ecrClient.AuthConfig},
		},
	}

	Profile := "default"
	val, ok := os.LookupEnv("AWS_PROFILE")
	if ok {
		Profile = val
	}
	if opts.regionalBuildMode && Profile != "default" {
		clients, err = clients.GetProfileSDKConnection("ecrpublic", Profile, ecrPublicRegion)
		if err != nil {
			BundleLog.Error(err, "Unable create SDK Client connections")
			os.Exit(1)
		}

		clients.ecrPublicClientRelease.SourceRegistry, err = clients.ecrPublicClientRelease.GetRegistryURI()
		if err != nil {
			BundleLog.Error(err, "Unable create find Public ECR URI for destination account")
			os.Exit(1)
		}
		dockerStruct.Auths["public.ecr.aws"] = DockerAuthRegistry{clients.ecrPublicClientRelease.AuthConfig}
	}

	clients.ecrPublicClient.SourceRegistry, err = clients.ecrPublicClient.GetRegistryURI()
	if err != nil {
		return fmt.Errorf("getting registry URI: %w", err)
	}
	dockerStruct.Auths["public.ecr.aws"] = DockerAuthRegistry{clients.ecrPublicClient.AuthConfig}

	dockerAuth, err := NewAuthFile(dockerStruct)
	if err != nil {
		return fmt.Errorf("creating auth file: %w", err)
	}
	if dockerAuth.Authfile == "" {
		return fmt.Errorf("no authfile generated")
	}
	clients.helmDriver, err = NewHelm(BundleLog, dockerAuth.Authfile)
	if err != nil {
		BundleLog.Error(err, "Unable to create Helm driver")
		os.Exit(1)
	}
	for repoName, versions := range promoteCharts {
		for _, version := range versions {
			err = clients.PromoteHelmChart(repoName, dockerAuth.Authfile, version, opts.copyImages, opts.regionalBuildMode)
			if err != nil {
				return fmt.Errorf("promoting Helm chart: %w", err)
			}
		}
	}
	err = dockerAuth.Remove()
	if err != nil {
		return fmt.Errorf("cleaning up docker auth file: %w", err)
	}
	BundleLog.Info("Promote Finished, exiting gracefully")
	return nil
}

func cmdRegion(opts *Options) error {
	BundleLog.Info("Starting Region Check Process")
	if opts.bundleFile == "" {
		BundleLog.Info("Please use the --bundle flag when running region check")
		os.Exit(1)
	}
	Bundle, err := ValidateBundle(opts.bundleFile)
	if err != nil {
		BundleLog.Error(err, "Unable to validate input file")
		os.Exit(1)
	}

	d := &RepositoryCloudWatch{}
	k8sVersionSlice := strings.Split(Bundle.ObjectMeta.Name, "-")
	K8sVersion := fmt.Sprintf("%s-%s", k8sVersionSlice[0], k8sVersionSlice[1])

	cloudWatchDataStruct := []RepositoryCloudWatch{}

	BundleLog.Info("Getting list of images to Region Check")
	for _, packages := range Bundle.Spec.Packages {
		for _, versions := range packages.Source.Versions {
			for _, images := range versions.Images {
				d = &RepositoryCloudWatch{
					Repository: images.Repository,
					Digest:     images.Digest,
					TotalHits:  0,
					K8sVersion: K8sVersion,
				}
				cloudWatchDataStruct = append(cloudWatchDataStruct, *d)
			}
		}
	}

	// Deduplicate the Array of structs for the CRDs packages which contain the same image reference twice.
	m := map[RepositoryCloudWatch]struct{}{}
	uniquecloudWatchDataStruct := []RepositoryCloudWatch{}
	for _, d := range cloudWatchDataStruct {
		if _, ok := m[d]; !ok {
			uniquecloudWatchDataStruct = append(uniquecloudWatchDataStruct, d)
			m[d] = struct{}{}
		}
	}

	// Creating AWS Clients with profile
	Profile := "default"
	val, ok := os.LookupEnv("AWS_PROFILE")
	if ok {
		Profile = val
	}
	BundleLog.Info("Using Env", "AWS_PROFILE", Profile)
	BundleLog.Info("Creating SDK connections to Region Check")
	clients := &SDKClients{}
	conf, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(ecrRegion),
		config.WithSharedConfigProfile(Profile),
	)
	if err != nil {
		return fmt.Errorf("loading default AWS config: %w", err)
	}
	cloudwatchC := cloudwatch.NewFromConfig(conf)
	clients, err = clients.GetProfileSDKConnection("ecr", Profile, ecrRegion)
	if err != nil {
		BundleLog.Error(err, "getting SDK connection")
		os.Exit(1)
	}
	clients, err = clients.GetProfileSDKConnection("sts", Profile, ecrRegion)
	if err != nil {
		BundleLog.Error(err, "getting profile SDK connection")
		os.Exit(1)
	}

	var cloudwatchData []cloudwatchtypes.MetricDatum
	var missingList []string
	for _, region := range RegionList {
		BundleLog.Info("Starting Check for", "Region", region)
		clients, err = clients.GetProfileSDKConnection("ecr", Profile, region)
		if err != nil {
			BundleLog.Error(err, "getting ECR SDK connection")
			os.Exit(1)
		}
		for i, image := range uniquecloudWatchDataStruct {
			check, err := clients.ecrClientRelease.shaExistsInRepository(image.Repository, image.Digest)
			if err != nil {
				BundleLog.Error(err, "finding ECR images")
			}
			if check {
				uniquecloudWatchDataStruct[i].TotalHits++
			} else {
				missingList = append(missingList, fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:%s", clients.stsClientRelease.AccountID, region, image.Repository, image.Digest))
			}
		}
	}
	BundleLog.Info("Missing Region List:")
	printSlice(missingList)
	for i, image := range uniquecloudWatchDataStruct {
		percent := (float64(image.TotalHits) / float64(len(RegionList))) * 100
		uniquecloudWatchDataStruct[i].Percent = percent
		cloudwatchData = FormCloudWatchData(cloudwatchData, image.Repository, uniquecloudWatchDataStruct[i].Percent)
	}
	err = PushCloudWatchRegionCheckData(cloudwatchC, cloudwatchData, uniquecloudWatchDataStruct[0].K8sVersion)
	if err != nil {
		BundleLog.Error(err, "pushing cloudwatch data")
		os.Exit(1)
	}
	BundleLog.Info("Finished Region Check!")
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
	// Creating AWS Clients with profile
	Profile := "default"
	val, ok := os.LookupEnv("AWS_PROFILE")
	if ok {
		Profile = val
	}
	BundleLog.Info("Using Env", "AWS_PROFILE", Profile)
	clients, err = clients.GetProfileSDKConnection("sts", Profile, ecrRegion)
	if err != nil {
		BundleLog.Error(err, "getting profile SDK connection")
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
				fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", clients.stsClientRelease.AccountID, ecrRegion): {
					clients.ecrClient.AuthConfig,
				},
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
		addOnBundleSpec, name, copyImages, err := clients.NewBundleFromInput(Inputs)
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

		// We will make a compound check for public and private profile after the launch once we want to stop
		// push packages to private ECR.
		if opts.publicProfile != "" {
			BundleLog.Info("Starting release public ECR process....")
			clients, err := GetSDKClients(opts.regionalBuildMode)
			if err != nil {
				BundleLog.Error(err, "getting sdk clients")
				os.Exit(1)
			}

			clients, err = clients.GetProfileSDKConnection("ecrpublic", opts.publicProfile, ecrPublicRegion)
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
					fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", clients.stsClient.AccountID, ecrRegion): {
						clients.ecrClient.AuthConfig,
					},
					fmt.Sprintf("public.ecr.aws/%s", clients.ecrPublicClient.SourceRegistry): {
						clients.ecrPublicClient.AuthConfig,
					},
					"public.ecr.aws": {clients.ecrPublicClientRelease.AuthConfig},
				},
			}
			dockerAuth, err = NewAuthFile(dockerReleaseStruct)
			if err != nil || dockerAuth.Authfile == "" {
				BundleLog.Error(err, "Unable create AuthFile")
				os.Exit(1)
			}
			clients.helmDriver, err = NewHelm(BundleLog, dockerAuth.Authfile)
			if err != nil {
				BundleLog.Error(err, "Unable to create Helm driver")
				os.Exit(1)
			}
			for _, charts := range addOnBundleSpec.Packages {
				for _, versions := range charts.Source.Versions {
					err = clients.PromoteHelmChart(charts.Source.Repository, dockerAuth.Authfile, versions.Name, copyImages[charts.Source.Repository], opts.regionalBuildMode)
					if err != nil {
						BundleLog.Error(err, "promoting helm chart",
							"name", charts.Source.Repository)
						os.Exit(1)
					}
				}
			}
			err = dockerAuth.Remove()
			if err != nil {
				BundleLog.Error(err, "removing docker auth file")
				os.Exit(1)
			}

			return nil
		}

		// See above comment about compound check when we want to cutover
		// if o.publicProfile != "" && if o.privateProfile != "" {}
		if opts.privateProfile != "" {
			BundleLog.Info("Starting release to private ECR process....")
			clients, err := GetSDKClients(opts.regionalBuildMode)
			if err != nil {
				BundleLog.Error(err, "getting SDK clients")
				os.Exit(1)
			}

			clients, err = clients.GetProfileSDKConnection("ecr", opts.privateProfile, ecrRegion)
			if err != nil {
				BundleLog.Error(err, "getting SDK connection")
				os.Exit(1)
			}
			clients, err = clients.GetProfileSDKConnection("sts", opts.privateProfile, ecrRegion)
			if err != nil {
				BundleLog.Error(err, "getting profile SDK connection")
				os.Exit(1)
			}
			dockerReleaseStruct = &DockerAuth{
				Auths: map[string]DockerAuthRegistry{
					fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", clients.stsClient.AccountID, ecrRegion): {
						clients.ecrClient.AuthConfig,
					},
					fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", clients.stsClientRelease.AccountID, ecrRegion): {
						clients.ecrClientRelease.AuthConfig,
					},
				},
			}
			dockerAuth, err = NewAuthFile(dockerReleaseStruct)
			if err != nil {
				BundleLog.Error(err, "Unable create AuthFile")
				os.Exit(1)
			}
			clients.helmDriver, err = NewHelm(BundleLog, dockerAuth.Authfile)
			if err != nil {
				BundleLog.Error(err, "Unable to create Helm driver")
				os.Exit(1)
			}
			for _, charts := range addOnBundleSpec.Packages {
				for _, versions := range charts.Source.Versions {
					err = clients.PromoteHelmChart(charts.Source.Repository, dockerAuth.Authfile, versions.Name, true, opts.regionalBuildMode)
					if err != nil {
						BundleLog.Error(err, "promoting helm chart",
							"name", charts.Source.Repository)
						os.Exit(1)
					}
				}
			}
			err = dockerAuth.Remove()
			if err != nil {
				BundleLog.Error(err, "removing docker auth file")
				os.Exit(1)
			}
			return nil
		}

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
