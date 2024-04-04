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
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/xeipuuv/gojsonschema"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	"github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/bundle"
)

type packageValidator struct {
	Client       client.Client
	BundleClient bundle.Client
	decoder      *admission.Decoder
}

func InitPackageValidator(mgr ctrl.Manager) error {
	mgr.GetWebhookServer().
		Register("/validate-packages-eks-amazonaws-com-v1alpha1-package",
			&webhook.Admission{Handler: &packageValidator{
				Client:       mgr.GetClient(),
				BundleClient: bundle.NewManagerClient(mgr.GetClient()),
				decoder:      admission.NewDecoder(mgr.GetScheme()),
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

	if p.Annotations["anywhere.eks.aws.com/internal"] == "true" {
		return admission.Response{AdmissionResponse: admissionv1.AdmissionResponse{Allowed: true}}
	}

	clusterName := p.GetClusterName()
	if clusterName == "" {
		clusterName = os.Getenv("CLUSTER_NAME")
	}

	activeBundle, err := v.BundleClient.GetActiveBundle(ctx, clusterName)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("getting PackageBundle: %v", err))
	}

	isConfigValid, err := v.isPackageValid(p, activeBundle)

	resp := &admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{Allowed: isConfigValid},
	}

	if !isConfigValid {
		reason := fmt.Sprintf("package %s failed validation with error: %v", p.Name, err)
		resp.AdmissionResponse.Result = &metav1.Status{
			Status:  metav1.StatusFailure,
			Code:    http.StatusBadRequest,
			Message: reason,
			Reason:  metav1.StatusReason(reason),
		}
	}

	return *resp
}

func (v *packageValidator) isPackageValid(p *v1alpha1.Package, activeBundle *v1alpha1.PackageBundle) (bool, error) {
	packageInBundle, err := activeBundle.FindPackage(p.Spec.PackageName)
	if err != nil {
		return false, err
	}

	version := p.Spec.PackageVersion
	if version == "" {
		version = v1alpha1.Latest
	}
	packageVersion, err := activeBundle.FindVersion(packageInBundle, version)
	if err != nil {
		return false, err
	}

	if packageInBundle.WorkloadOnly && !p.IsInstalledOnWorkload() {
		return false, fmt.Errorf("package %s should only be installed on a workload cluster", p.Name)
	}

	jsonSchema, err := packageInBundle.GetJsonSchema(&packageVersion)
	if err != nil {
		return false, err
	}

	result, err := validatePackage(p, jsonSchema)
	if err != nil {
		return false, fmt.Errorf(err.Error())
	}

	if p.Status.Spec.TargetNamespace != "" && p.Status.Spec.TargetNamespace != p.Spec.TargetNamespace {
		return false, fmt.Errorf("package %s targetNamespace is immutable", p.Name)
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

	packageConfigBytes, err := yaml.YAMLToJSON([]byte(p.Spec.Config))
	if err != nil {
		return nil, fmt.Errorf("error converting package configurations to yaml %v", err)
	}

	packageConfigString := string(packageConfigBytes)
	if p.Spec.Config == "" {
		packageConfigString = "{}"
	}

	configToValidate := gojsonschema.NewStringLoader(packageConfigString)

	return schema.Validate(configToValidate)
}
