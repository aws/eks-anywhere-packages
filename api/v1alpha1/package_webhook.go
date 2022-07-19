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

package v1alpha1

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/xeipuuv/gojsonschema"
	"io/ioutil"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"
)

type packageValidator struct {
	Client  client.Client
	decoder *admission.Decoder
}

// apilog is for logging in this package.
var packagelog = ctrl.Log.WithName("api")

func InitPackageValidator(mgr ctrl.Manager) error {
	fmt.Println("Package Validator is getting requested!")
	mgr.GetWebhookServer().
		Register("/validate-packages-eks-amazonaws-com-v1alpha1-package",
			&webhook.Admission{Handler: &packageValidator{
				Client: mgr.GetClient(),
			}})
	return nil
}

func (v *packageValidator) Handle(ctx context.Context, request admission.Request) admission.Response {
	fmt.Println("Package Validator is getting requested from Handle!")
	p := &Package{}
	err := v.decoder.Decode(request, p)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("decoding request: %w", err))
	}

	bundles := &PackageBundleList{}
	err = v.Client.List(ctx, bundles, &client.ListOptions{Namespace: PackageNamespace})
	if err != nil {
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("listing package bundles: %w", err))
	}

	activeBundle := bundles.Items[0]
	for _, bundle := range bundles.Items {
		if bundle.Status.State == PackageBundleStateActive {
			activeBundle = bundle
		}
	}

	isConfigValid, err := v.isPackageConfigValid(p, &activeBundle)

	resp := &admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{Allowed: isConfigValid},
	}

	if !isConfigValid {
		msg := fmt.Sprintf("package %s failed validation with error: %v", p.Name, err)
		resp.AdmissionResponse.Result = &metav1.Status{
			Status:  metav1.StatusFailure,
			Code:    http.StatusBadRequest,
			Message: msg,
			Reason:  metav1.StatusReasonNotFound,
		}
	}

	return *resp
}

func (v *packageValidator) isPackageConfigValid(p *Package, activeBundle *PackageBundle) (bool, error) {
	packageInBundle, err := getPackageInBundle(activeBundle, p.Spec.PackageName)

	if err != nil {
		return false, err
	}

	configuration := packageInBundle.Source.Versions[0].Configurations[0].Default
	decodedConfiguration, err := base64.StdEncoding.DecodeString(configuration)

	if err != nil {
		return false, err
	}

	reader := bytes.NewReader(decodedConfiguration)

	gzreader, err := gzip.NewReader(reader)
	output, err := ioutil.ReadAll(gzreader)
	jsonSchema, err := yaml.YAMLToJSON(output)

	sl := gojsonschema.NewSchemaLoader()
	loader := gojsonschema.NewStringLoader(string(jsonSchema))
	packagelog.Info("Schema: " + string(jsonSchema))

	schema, err := sl.Compile(loader)

	packageConfig, err := yaml.YAMLToJSON([]byte(p.Spec.Config))
	configToValidate := gojsonschema.NewStringLoader(string(packageConfig))
	packagelog.Info("Config: " + string(packageConfig))

	result, err := schema.Validate(configToValidate)
	if err != nil {
		return false, fmt.Errorf("error validating configurations %v", err.Error())
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

func getPackageInBundle(activeBundle *PackageBundle, packageName string) (*BundlePackage, error) {
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
