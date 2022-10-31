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
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/bundle"
	"github.com/aws/eks-anywhere-packages/pkg/signature"
)

const (
	PublicKeyEnvVar = "EKSA_PUBLIC_KEY"
)

type packageBundleValidator struct {
	Client       client.Client
	BundleClient bundle.Client
	decoder      *admission.Decoder
	log          logr.Logger
}

func NewPackageBundleValidator(mgr ctrl.Manager) packageBundleValidator {
	client := mgr.GetClient()
	return packageBundleValidator{
		Client:       client,
		BundleClient: bundle.NewPackageBundleClient(client),
		log:          mgr.GetLogger().WithName("webhook"),
	}
}

func InitPackageBundleValidator(mgr ctrl.Manager) error {
	handler := NewPackageBundleValidator(mgr)
	mgr.GetWebhookServer().
		Register("/validate-packages-eks-amazonaws-com-v1alpha1-packagebundle",
			&webhook.Admission{Handler: &handler})
	return nil
}

func (v *packageBundleValidator) Handle(_ context.Context, request admission.Request) admission.Response {
	pb := &v1alpha1.PackageBundle{}
	err := v.decoder.Decode(request, pb)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("decoding request: %w", err))
	}

	err = v.isPackageBundleValid(pb)

	resp := &admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{Allowed: err == nil},
	}

	if err != nil {
		reason := fmt.Sprintf("package %s failed validation with error: %v", pb.Name, err.Error())
		resp.AdmissionResponse.Result = &metav1.Status{
			Status:  metav1.StatusFailure,
			Code:    http.StatusBadRequest,
			Message: reason,
			Reason:  metav1.StatusReason(reason),
		}
	}

	return *resp
}

func (v *packageBundleValidator) isPackageBundleValid(pb *v1alpha1.PackageBundle) error {
	if !pb.IsValidVersion() {
		v.log.Info("Invalid bundle name (should be in the format vx-xx-xxxx where x is a digit): " + pb.Name)
		return fmt.Errorf("Invalid bundle name (should be in the format vx-xx-xxxx where x is a digit): " + pb.Name)
	}

	keyOverride := os.Getenv(PublicKeyEnvVar)
	domain := signature.EksaDomain
	if keyOverride != "" {
		domain = signature.Domain{Name: signature.DomainName, Pubkey: keyOverride}
	}
	valid, digest, yml, err := signature.ValidateSignature(pb, domain)
	if err != nil {
		return err
	}
	if !valid {
		v.log.Info("Invalid signature", "Error", err, "Digest", base64.StdEncoding.EncodeToString(digest[:]), "Manifest", string(yml))
		return fmt.Errorf("The signature is invalid for the configured public key: " + domain.Pubkey)
	}
	return nil
}

// InjectDecoder injects the decoder.
func (v *packageBundleValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
