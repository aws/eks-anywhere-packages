// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhook

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/aws/eks-anywhere-packages/pkg/bundle"
	"io/ioutil"
	"net/http"

	"github.com/xeipuuv/gojsonschema"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	"github.com/aws/eks-anywhere-packages/api/v1alpha1"
)

type packageValidator struct {
	Client  client.Client
	decoder *admission.Decoder
}

func InitPackageValidator(mgr ctrl.Manager) error {
	mgr.GetWebhookServer().
		Register("/validate-packages-eks-amazonaws-com-v1alpha1-package",
			&webhook.Admission{Handler: &packageValidator{
				Client: mgr.GetClient(),
			}})
	return nil
}

func (v *packageValidator) Handle(ctx context.Context, request admission.Request) admission.Response {
	p := &v1alpha1.Package{}
	err := v.decoder.Decode(request, p)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("decoding request: %w", err))
	}

	bundleClient := bundle.NewPackageBundleClient(v.Client)
	activeBundle, err := bundleClient.GetActiveBundle(ctx)

	if err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("getting PackageBundle: %v", err))
	}

	isConfigValid, err := v.isPackageConfigValid(p, activeBundle)

	resp := &admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{Allowed: isConfigValid},
	}

	if !isConfigValid {
		msg := fmt.Sprintf("package %s failed validation with error: %v", p.Name, err)
		resp.AdmissionResponse.Result = &metav1.Status{
			Status:  metav1.StatusFailure,
			Code:    http.StatusBadRequest,
			Message: msg,
			Reason:  metav1.StatusReasonBadRequest,
		}
	}

	return *resp
}

func (v *packageValidator) getActiveBundle(ctx context.Context, b string) (*v1alpha1.PackageBundle, error) {
	nn := types.NamespacedName{
		Namespace: v1alpha1.PackageNamespace,
		Name:      b,
	}
	activeBundle := &v1alpha1.PackageBundle{}
	err := v.Client.Get(ctx, nn, activeBundle)
	if err != nil {
		return nil, fmt.Errorf("error fetching bundle %s %w", b, err)
	}
	return activeBundle, nil
}

func (v *packageValidator) getActiveController(ctx context.Context) (v1alpha1.PackageBundleController, error) {
	pbc := v1alpha1.PackageBundleController{}
	key := types.NamespacedName{
		Namespace: v1alpha1.PackageNamespace,
		Name:      v1alpha1.PackageBundleControllerName,
	}
	err := v.Client.Get(ctx, key, &pbc)
	return pbc, err
}

func (v *packageValidator) isPackageConfigValid(p *v1alpha1.Package, activeBundle *v1alpha1.PackageBundle) (bool, error) {
	packageInBundle, err := getPackageInBundle(activeBundle, p.Spec.PackageName)
	if err != nil {
		return false, err
	}

	packageVersions := packageInBundle.Source.Versions
	if len(packageVersions) < 1 {
		return false, fmt.Errorf("package %s does not contain any versions", p.Name)
	}

	jsonSchema, err := getPackagesJsonSchema(packageInBundle)
	if err != nil {
		return false, err
	}

	result, err := validatePackage(p, jsonSchema)
	if err != nil {
		return false, fmt.Errorf(err.Error())
	}

	b := new(bytes.Buffer)
	if !result.Valid() {
		for _, e := range result.Errors() {
			fmt.Fprintf(b, "- %s\n", e)
		}
		return false, fmt.Errorf("error validating configurations %s", b.String())
	}

	return true, nil
}

func validatePackage(p *v1alpha1.Package, jsonSchema []byte) (*gojsonschema.Result, error) {
	sl := gojsonschema.NewSchemaLoader()
	loader := gojsonschema.NewStringLoader(string(jsonSchema))
	schema, err := sl.Compile(loader)
	if err != nil {
		return nil, fmt.Errorf("error compiling schema %v", err)
	}

	packageConfig, err := yaml.YAMLToJSON([]byte(p.Spec.Config))
	if err != nil {
		return nil, fmt.Errorf("error converting package configurations to yaml %v", err)
	}
	configToValidate := gojsonschema.NewStringLoader(string(packageConfig))

	return schema.Validate(configToValidate)
}

func getPackagesJsonSchema(bundlePackage *v1alpha1.BundlePackage) ([]byte, error) {
	// The package configuration is gzipped and base64 encoded
	// When processing the configuration, the reverse occurs: base64 decode, then unzip
	configuration := bundlePackage.Source.Versions[0].Configurations[0].Default
	decodedConfiguration, err := base64.StdEncoding.DecodeString(configuration)
	if err != nil {
		return nil, fmt.Errorf("error decoding configurations %v", err)
	}

	reader := bytes.NewReader(decodedConfiguration)
	gzreader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("error when uncompressing configurations %v", err)
	}

	output, err := ioutil.ReadAll(gzreader)
	if err != nil {
		return nil, fmt.Errorf("error reading configurations %v", err)
	}

	return output, nil
}

func getPackageInBundle(activeBundle *v1alpha1.PackageBundle, packageName string) (*v1alpha1.BundlePackage, error) {
	for _, p := range activeBundle.Spec.Packages {
		if p.Name == packageName {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("package %s not found", packageName)
}

// InjectDecoder injects the decoder.
func (v *packageValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
