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
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type activeBundleValidator struct {
	Client  client.Client
	decoder *admission.Decoder
}

func InitActiveBundleValidator(mgr ctrl.Manager) error {
	mgr.GetWebhookServer().
		Register("/validate-packages-eks-amazonaws-com-v1alpha1-packagebundlecontroller",
			&webhook.Admission{Handler: &activeBundleValidator{
				Client: mgr.GetClient(),
			}})
	return nil
}

func (v *activeBundleValidator) Handle(ctx context.Context,
	req admission.Request) admission.Response {

	pbc := &PackageBundleController{}
	err := v.decoder.Decode(req, pbc)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("decoding request: %w", err))
	}

	bundles := &PackageBundleList{}
	err = v.Client.List(ctx, bundles)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("listing package bundles: %w", err))
	}

	resp, err := v.handleInner(ctx, pbc, bundles)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return *resp
}

func (v *activeBundleValidator) handleInner(ctx context.Context,
	pbc *PackageBundleController, bundles *PackageBundleList) (
	*admission.Response, error) {

	found := false
	for _, bundle := range bundles.Items {
		if bundle.Name == pbc.Spec.ActiveBundle {
			found = true
			break
		}
	}

	resp := &admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{Allowed: found},
	}

	if !found {
		msg := fmt.Sprintf("package bundle not found with name: %q", pbc.Spec.ActiveBundle)
		resp.AdmissionResponse.Result = &metav1.Status{
			Status:  metav1.StatusFailure,
			Code:    http.StatusNotFound,
			Message: msg,
			Reason:  metav1.StatusReasonNotFound,
		}
	}

	return resp, nil
}

var _ admission.DecoderInjector = (*activeBundleValidator)(nil)

// InjectDecoder injects the decoder.
func (v *activeBundleValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
