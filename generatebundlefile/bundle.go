package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	sig "github.com/aws/eks-anywhere-packages/pkg/signature"
)

const (
	// Excludes .spec.packages[].source.registry .spec.packages[].source.repository
	Excludes = "LnNwZWMucGFja2FnZXNbXS5zb3VyY2UucmVnaXN0cnkKLnNwZWMucGFja2FnZXNbXS5zb3VyY2UucmVwb3NpdG9yeQo="
)

var (
	FullSignatureAnnotation   = path.Join(sig.EksaDomain.Name, sig.SignatureAnnotation)
	FullExcludesAnnotation    = path.Join(sig.EksaDomain.Name, sig.ExcludesAnnotation)
	DefaultExcludesAnnotation = map[string]string{
		FullExcludesAnnotation: Excludes,
	}
)
var generatedMetadataFields = []string{"creationTimestamp", "generation", "managedFields", "uid", "resourceVersion"}

type BundleGenerateOpt func(config *BundleGenerate)

// NewBundleGenerate is used for generating YAML for generating a new sample CRD file.
func NewBundleGenerate(bundleName string, opts ...BundleGenerateOpt) *api.PackageBundle {
	annotations := make(map[string]string)
	annotations[FullExcludesAnnotation] = Excludes
	return &api.PackageBundle{
		TypeMeta: metav1.TypeMeta{
			Kind:       api.PackageBundleKind,
			APIVersion: api.SchemeBuilder.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        bundleName,
			Namespace:   api.PackageNamespace,
			Annotations: DefaultExcludesAnnotation,
		},
		Spec: api.PackageBundleSpec{
			Packages: []api.BundlePackage{
				{
					Name: "sample-package",
					Source: api.BundlePackageSource{
						Repository: "sample-Repository",
						Versions: []api.SourceVersion{
							{
								Name:   "v0.0",
								Digest: "sha256:da25f5fdff88c259bb2ce7c0f1e9edddaf102dc4fb9cf5159ad6b902b5194e66",
							},
						},
					},
				},
			},
		},
	}
}

// NewPackageFromInput finds the SHA tags for any images in your BundlePackage
func (c *SDKClients) NewPackageFromInput(project Project) (*api.BundlePackage, error) {
	var versionList []api.SourceVersion
	var err error
	// Check bundle Input registry for ECR Public Registry
	if strings.Contains(project.Registry, "public.ecr.aws") {
		versionList, err = c.GetShaForPublicInputs(project)
		if err != nil {
			return nil, err
		}
	}
	// Check bundle Input registry for ECR Private Registry
	if strings.Contains(project.Registry, "amazonaws.com") {
		versionList, err = c.ecrClient.GetShaForInputs(project)
		if err != nil {
			return nil, err
		}
	}
	if len(versionList) < 1 {
		return nil, fmt.Errorf("unable to find SHA sum for given input tag %v", project.Versions)
	}
	bundlePkg := &api.BundlePackage{
		Name:         project.Name,
		WorkloadOnly: project.WorkloadOnly,
		Source: api.BundlePackageSource{
			Repository: project.Repository,
			Registry:   project.Registry,
			Versions:   versionList,
		},
	}
	return bundlePkg, nil
}

// AddMetadata adds the corresponding metadata to the crd files.
func AddMetadata(s api.PackageBundleSpec, name string) *api.PackageBundle {
	return &api.PackageBundle{
		TypeMeta: metav1.TypeMeta{
			Kind:       api.PackageBundleKind,
			APIVersion: api.SchemeBuilder.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   api.PackageNamespace,
			Annotations: DefaultExcludesAnnotation,
		},
		Spec: s,
	}
}

// IfSignature checks if a signature exsits on a Packagebundle
func IfSignature(bundle *api.PackageBundle) (bool, error) {
	annotations := bundle.Annotations
	if annotations != nil {
		return true, nil
	}
	return false, nil
}

// CheckSignature checks if current signature is equal to signature to added as an annotation, and skips if they are the same.
func CheckSignature(bundle *api.PackageBundle, signature string) (bool, error) {
	if signature == "" || bundle == nil {
		return false, fmt.Errorf("either signature or bundle is missing, but are required")
	}
	annotations := map[string]string{
		FullSignatureAnnotation: signature,
	}
	//  If current signature on file isn't at the --signature input return false, otherwsie true
	if annotations[FullSignatureAnnotation] != bundle.Annotations[FullSignatureAnnotation] {
		return false, fmt.Errorf("A signature already exists on the input file signatue")
	}
	return true, nil
}

// GetBundleSignature calls KMS and retrieves a signature, then base64 decodes it and returns that back
func GetBundleSignature(ctx context.Context, bundle *api.PackageBundle, key string) (string, error) {
	digest, _, err := sig.GetDigest(bundle, sig.EksaDomain)
	if err != nil {
		return "", err
	}

	// Creating AWS Clients with profile
	ConfigFilePath := "~/.aws/config"
	val, ok := os.LookupEnv("AWS_CONFIG_FILE")
	if ok {
		ConfigFilePath = val
	}
	BundleLog.Info("Using Config File", "AWS_CONFIG_FILE", ConfigFilePath)
	confWithProfile, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigFiles(
			[]string{ConfigFilePath},
		),
		config.WithRegion("us-west-2"),
		config.WithSharedConfigProfile("default"),
	)
	if err != nil {
		fmt.Println("configuration error, " + err.Error())
		os.Exit(-1)
	}
	client := kms.NewFromConfig(confWithProfile)

	input := &kms.SignInput{
		KeyId:            &key,
		SigningAlgorithm: types.SigningAlgorithmSpecEcdsaSha256,
		MessageType:      types.MessageTypeDigest,
		Message:          digest[:],
	}
	out, err := client.Sign(context.Background(), input)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(out.Signature), nil
}

func serializeBundle(bundle *api.PackageBundle) ([]byte, error) {
	out, err := json.Marshal(bundle)
	if err != nil {
		return nil, err
	}
	raw := make(map[string]interface{})
	err = json.Unmarshal(out, &raw)
	if err != nil {
		return nil, err
	}
	delete(raw, "status")
	meta := raw["metadata"].(map[string]interface{})
	for _, f := range generatedMetadataFields {
		delete(meta, f)
	}
	return yaml.Marshal(raw)
}
