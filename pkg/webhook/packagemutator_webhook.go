package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	"github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/bundle"
	"github.com/aws/eks-anywhere-packages/pkg/types"
)

type packageMutator struct {
	Client       client.Client
	BundleClient bundle.Client
	decoder      *admission.Decoder
}

func InitPackageMutator(mgr ctrl.Manager) error {
	mgr.GetWebhookServer().
		Register("/mutate-packages-eks-amazonaws-com-v1alpha1-package",
			&webhook.Admission{Handler: &packageMutator{
				Client:       mgr.GetClient(),
				BundleClient: bundle.NewPackageBundleClient(mgr.GetClient()),
			}})
	return nil
}

func (m *packageMutator) Handle(ctx context.Context, request admission.Request) admission.Response {
	p := &v1alpha1.Package{}
	err := m.decoder.Decode(request, p)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("decoding request: %w", err))
	}

	activeBundle, err := m.BundleClient.GetActiveBundle(ctx)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("getting PackageBundle: %v", err))
	}
	packageInBundle, err := activeBundle.GetPackageFromBundle(p.Spec.PackageName)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("getting package from bundle: %v", err))
	}
	jsonSchema, err := packageInBundle.GetJsonSchema()
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("getting json schema for package: %v", err))
	}

	err = setDefaults(p, jsonSchema)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("setting defaults: %w", err))
	}
	newPackage, err := json.Marshal(p)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("marshalling updating configurations to json: %v", err))
	}
	return admission.PatchResponseFromRaw(request.Object.Raw, newPackage)
}

func setDefaults(p *v1alpha1.Package, jsonSchema []byte) error {
	currentConfigs, err := p.GetValues()
	if err != nil {
		return err
	}

	// Convert json schema to a GO struct
	schemaObj := &types.Schema{}
	err = json.Unmarshal(jsonSchema, schemaObj)
	if err != nil {
		return fmt.Errorf("unmarshalling schema to an object %v", err)
	}

	for key, val := range schemaObj.Properties {
		keySegments := strings.Split(key, ".")
		updateDefault(currentConfigs, keySegments, 0, val.Default)
	}

	// After updating defaults, marshal the new configs in order to update the packages configuration
	updatedConfigs, err := yaml.Marshal(currentConfigs)
	if err != nil {
		return fmt.Errorf("marshalling updated configurations to yaml %v", err)
	}
	p.Spec.Config = string(updatedConfigs)
	return nil
}

func updateDefault(values map[string]interface{}, keySegments []string, index int, val string) {
	// If no more data to process, consider all the defaults are set
	if index >= len(keySegments) {
		return
	}

	key := keySegments[index]

	// When there is a single data point, no need to continue
	// e.g. expose.tls.auto, no need to continue further when processing auto
	if index == len(keySegments)-1 {
		// We only need to update the values when the key doesn't already exist
		if _, ok := values[key]; !ok {
			if bVal, err := strconv.ParseBool(val); err == nil {
				values[key] = bVal
			} else {
				values[key] = val
			}
		}
		return
	}

	// Unique case, when there is a malformed package that doesn't conform to the json schema
	// i.e. package contains expose: tls: "value" but json schema contains expose.tls.auto
	if _, ok := values[key].(string); ok {
		return
	}

	// We only need to update the values when the key doesn't exist
	if _, ok := values[key]; !ok {
		values[key] = map[string]interface{}{}
	}

	updateDefault(values[key].(map[string]interface{}), keySegments, index+1, val)
}

// InjectDecoder injects the decoder.
func (m *packageMutator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}
