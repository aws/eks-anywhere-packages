package main

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
	"sigs.k8s.io/yaml"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/bundle"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "packages.eks.amazonaws.com", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// Default version if one is not specified on the input
	DefaultKubernetesVersion = "1.21"
)

// +kubebuilder:object:generate=false
type BundleGenerateOpt func(config *BundleGenerate)

// Used for generating yaml for generating a new crd sample file
func NewBundleGenerate(bundleName string, opts ...BundleGenerateOpt) *api.PackageBundle {
	clusterConfig := &api.PackageBundle{
		TypeMeta: metav1.TypeMeta{
			Kind:       api.PackageBundleKind,
			APIVersion: SchemeBuilder.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundleName,
			Namespace: bundle.ActiveBundleNamespace,
		},
		Spec: api.PackageBundleSpec{
			KubeVersion: DefaultKubernetesVersion,
			Packages: []api.BundlePackage{
				{
					Name: "sample-package",
					Source: api.BundlePackageSource{
						Registry:   "sample-Registry",
						Repository: "sample-Repository",
						Versions: []api.SourceVersion{
							{
								Name: "v0.0",
								Tag:  "sha256:da25f5fdff88c259bb2ce7c0f1e9edddaf102dc4fb9cf5159ad6b902b5194e66",
							},
						},
					},
				},
			},
		},
	}
	return clusterConfig
}

// NewPackageFromInput finds the SHA tags for any images in your BundlePackage
func (projects Project) NewPackageFromInput() (*api.BundlePackage, error) {
	ecrClient, err := NewECRClient()
	if err != nil {
		return nil, err
	}
	versionList, err := ecrClient.GetShaForTags(projects)
	if err != nil {
		return nil, err
	}
	bundlePkg := &api.BundlePackage{
		Name: projects.Name,
		Source: api.BundlePackageSource{
			Registry:   projects.Registry,
			Repository: projects.Repository,
			Versions:   versionList,
		},
	}
	return bundlePkg, nil
}

// AddMetadata adds the corresponding metadata to the crd files.
func AddMetadata(s api.PackageBundleSpec) BundleGenerate {
	return BundleGenerate{
		TypeMeta: metav1.TypeMeta{
			Kind:       api.PackageBundleKind,
			APIVersion: SchemeBuilder.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.KubeVersion,
			Namespace: bundle.ActiveBundleNamespace,
		},
		Spec: s,
	}
}

// MarshalBundleSpec will create yaml objects from bundlespecs
func (bundleSpec BundleGenerate) MarshalBundleSpec() ([]byte, error) {
	marshallables := []interface{}{bundleSpec}
	resources := make([][]byte, len(marshallables))
	for _, marshallable := range marshallables {
		resource, err := yaml.Marshal(marshallable)
		if err != nil {
			return nil, fmt.Errorf("failed marshalling resource for bundle spec: %v", err)
		}
		resources = append(resources, resource)
	}
	return ConcatYamlResources(resources...), nil
}

// WriteBundleConfig writes the yaml objects to files in your defined dir
func WriteBundleConfig(pkgBundleSpec api.PackageBundleSpec, writer FileWriter) error {
	bundle := AddMetadata(pkgBundleSpec)
	crdContent, err := bundle.MarshalBundleSpec()
	if err != nil {
		return err
	}
	if filePath, err := writer.Write(fmt.Sprintf("bundle-%s.yaml", pkgBundleSpec.KubeVersion), crdContent, PersistentFile); err != nil {
		err = fmt.Errorf("error writing bundle crd file into %s: %v", filePath, err)
		return err
	}
	return nil
}
