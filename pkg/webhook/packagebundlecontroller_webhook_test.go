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
	"encoding/json"
	"fmt"
	"log"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/authenticator/mocks"
)

func TestHandleInner(t *testing.T) {
	ctx := context.Background()

	t.Run("validates successfully", func(t *testing.T) {
		tcc := mocks.NewMockTargetClusterClient(gomock.NewController(t))
		tcc.EXPECT().GetServerVersion(gomock.Any(), gomock.Any()).Return(&version.Info{Major: "1", Minor: "21"}, nil)
		v := &activeBundleValidator{
			tcc: tcc,
		}
		pbc := &v1alpha1.PackageBundleController{
			Spec: v1alpha1.PackageBundleControllerSpec{
				ActiveBundle: "v1-21-1001",
			},
		}
		bundles := &v1alpha1.PackageBundleList{
			Items: []v1alpha1.PackageBundle{
				{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name: "v1-21-1001",
					},
					Spec:   v1alpha1.PackageBundleSpec{},
					Status: v1alpha1.PackageBundleStatus{},
				},
			},
		}
		resp, err := v.handleInner(ctx, pbc, bundles)
		if assert.NoError(t, err) {
			assert.NotNil(t, resp)
			assert.True(t, resp.AdmissionResponse.Allowed)
		}
	})

	t.Run("invalidates successfully", func(t *testing.T) {
		tcc := mocks.NewMockTargetClusterClient(gomock.NewController(t))
		tcc.EXPECT().GetServerVersion(gomock.Any(), gomock.Any()).Return(&version.Info{Major: "1", Minor: "20"}, nil)
		v := &activeBundleValidator{
			tcc: tcc,
		}
		pbc := &v1alpha1.PackageBundleController{
			Spec: v1alpha1.PackageBundleControllerSpec{
				ActiveBundle: "v1-21-1001",
			},
		}
		bundles := &v1alpha1.PackageBundleList{
			Items: []v1alpha1.PackageBundle{
				{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name: "v1-21-1001",
					},
					Spec:   v1alpha1.PackageBundleSpec{},
					Status: v1alpha1.PackageBundleStatus{},
				},
			},
		}
		resp, err := v.handleInner(ctx, pbc, bundles)
		if assert.NoError(t, err) {
			assert.NotNil(t, resp)
			assert.False(t, resp.AdmissionResponse.Allowed)
		}
	})

	t.Run("handles decoder errors", func(t *testing.T) {
		v := &activeBundleValidator{}
		req := admission.Request{
			AdmissionRequest: admissionv1.AdmissionRequest{
				Object: runtime.RawExtension{
					Raw: nil,
				},
			},
		}
		resp := v.Handle(ctx, req)
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "decoding request")
	})

	t.Run("handles list errors", func(t *testing.T) {
		pbc := &v1alpha1.PackageBundleController{
			Spec: v1alpha1.PackageBundleControllerSpec{
				ActiveBundle: "v1-21-1001",
			},
			Status: v1alpha1.PackageBundleControllerStatus{},
		}
		pbcBytes, err := json.Marshal(pbc)
		require.NoError(t, err)
		scheme := runtime.NewScheme()
		require.NoError(t, v1alpha1.AddToScheme(scheme))
		decoder := admission.NewDecoder(scheme)
		require.NoError(t, err)
		v := &activeBundleValidator{
			decoder: decoder,
			Client:  newFakeClient(scheme),
		}
		req := admission.Request{
			AdmissionRequest: admissionv1.AdmissionRequest{
				Object: runtime.RawExtension{
					Raw: pbcBytes,
				},
			},
		}
		resp := v.Handle(ctx, req)
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "listing package bundles: testing error")
	})

	t.Run("rejects unknown bundle names", func(t *testing.T) {
		v := &activeBundleValidator{}
		pbc := &v1alpha1.PackageBundleController{
			Spec: v1alpha1.PackageBundleControllerSpec{
				ActiveBundle: "v1-21-1002",
			},
		}
		bundles := &v1alpha1.PackageBundleList{
			Items: []v1alpha1.PackageBundle{
				{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name: "v1-21-1001",
					},
					Spec:   v1alpha1.PackageBundleSpec{},
					Status: v1alpha1.PackageBundleStatus{},
				},
			},
		}
		resp, err := v.handleInner(ctx, pbc, bundles)
		if assert.NoError(t, err) {
			assert.False(t, resp.AdmissionResponse.Allowed)
			assert.Equal(t, metav1.StatusFailure, resp.AdmissionResponse.Result.Status)
			assert.Equal(t, "activeBundle \"v1-21-1002\" not present on cluster", string(resp.AdmissionResponse.Result.Reason))
		}
	})
}

//
// Test helpers
//

type fakeClient struct {
	client.WithWatch
}

// var _ client.WithWatch = (*fakeClient)(nil)
func newFakeClient(scheme *runtime.Scheme) *fakeClient {
	fake := clientfake.NewClientBuilder().WithScheme(scheme).Build()
	return &fakeClient{
		WithWatch: fake,
	}
}

func (c *fakeClient) List(ctx context.Context,
	obj client.ObjectList, opts ...client.ListOption,
) error {
	log.Printf("list has been called")
	return fmt.Errorf("testing error")
}
