package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"path"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	sig "github.com/aws/eks-anywhere-packages/pkg/signature"
)

const (
	//.spec.packages[].source.registry
	//.spec.packages[].source.repository
	Excludes = "LnNwZWMucGFja2FnZXNbXS5zb3VyY2UucmVnaXN0cnkKLnNwZWMucGFja2FnZXNbXS5zb3VyY2UucmVwb3NpdG9yeQo="
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "packages.eks.amazonaws.com", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	FullSignatureAnnotation   = path.Join(sig.EksaDomain.Name, sig.SignatureAnnotation)
	FullExcludesAnnotation    = path.Join(sig.EksaDomain.Name, sig.ExcludesAnnotation)
	DefaultExcludesAnnotation = map[string]string{
		FullExcludesAnnotation: Excludes,
	}
)

// +kubebuilder:object:generate=false
type BundleGenerateOpt func(config *BundleGenerate)

// NewBundleGenerate is used for generating YAML for generating a new sample CRD file.
func NewBundleGenerate(bundleName string, opts ...BundleGenerateOpt) *api.PackageBundle {
	annotations := make(map[string]string)
	annotations[FullExcludesAnnotation] = Excludes
	return &api.PackageBundle{
		TypeMeta: metav1.TypeMeta{
			Kind:       api.PackageBundleKind,
			APIVersion: SchemeBuilder.GroupVersion.String(),
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
						Registry:   "sample-Registry",
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
func (c *ecrPublicClient) NewPackageFromInput(project Project) (*api.BundlePackage, error) {
	versionList, err := c.GetShaForInputs(project)
	if err != nil {
		return nil, err
	}
	if len(versionList) < 1 {
		return nil, fmt.Errorf("unable to find SHA sum for given input tag %v", project.Versions)
	}
	bundlePkg := &api.BundlePackage{
		Name: project.Name,
		Source: api.BundlePackageSource{
			Registry:   project.Registry,
			Repository: project.Repository,
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
			APIVersion: SchemeBuilder.GroupVersion.String(),
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
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic("configuration error, " + err.Error())
	}

	client := kms.NewFromConfig(cfg)

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
