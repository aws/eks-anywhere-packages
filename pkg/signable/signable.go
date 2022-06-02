package signable

import (
	"crypto"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	goyaml "gopkg.in/yaml.v2"
	"sigs.k8s.io/yaml"

	"github.com/aws/eks-anywhere-packages/pkg/signature"
)

const (
	DefaultHash = signature.DefaultHash
	ExcludesSep = signature.ExcludesSep
)

// Signable knows how to sign and output Kubernetes objects for EKS Anywhere
// Packages.
//
// This means knowing how to prepare the object for signing, how to sign it, and
// how to then output a signed version.
type Signable interface {
	// SignedYAML annotates the object's metadata with a signature.
	//
	// It obeys EKS Anywhere-specific annotations such as eksa.aws.com/exclude.
	SignedYAML(pemPrivateKey []byte) ([]byte, error)

	// SignedYAMLWithHash works like SignedYAML, but allows the caller to specify
	// the hash used for the signed digest.
	SignedYAMLWithHash(pemPrivateKey []byte, hash crypto.Hash) ([]byte, error)
}

// signable implements Signable.
type signable struct {
	Object
	// sigs.k8s.io/yaml doesn't re-export these, but they're needed.
	goyaml.Marshaler
	goyaml.Unmarshaler
}

var _ Signable = (*signable)(nil)

func New(object Object) *signable {
	return &signable{Object: object}
}

type Object interface {
	GetAnnotations() map[string]string
	SetAnnotations(map[string]string)
}

func (s signable) SignedYAML(pemPrivateKey []byte) ([]byte, error) {
	return s.SignedYAMLWithHash(pemPrivateKey, DefaultHash)
}

func (s signable) SignedYAMLWithHash(pemPrivateKey []byte, hash crypto.Hash) (
	[]byte, error) {

	yamlObject, err := s.patchedYAML(s.signingPatch)
	if err != nil {
		return nil, err
	}

	sig, err := SignDigestBase64(pemPrivateKey, yamlObject, hash)
	if err != nil {
		return nil, err
	}

	as := s.GetAnnotations()
	if as == nil {
		as = make(map[string]string)
		s.SetAnnotations(as)
	}
	as[signature.FullSignatureAnnotation] = sig

	return s.patchedYAML(s.exportPatch)
}

// patchedYAML applies a patch to itself, returning the resulting YAML.
//
// This is done by marshaling to JSON, applying the patch, then converting the
// JSON to YAML, which is similar to what Kustomize does. The primary difference
// is in the flags passed when applying the patch.
func (s signable) patchedYAML(fn patchFunction) ([]byte, error) {
	jsonObject, err := json.Marshal(s.Object)
	if err != nil {
		return nil, fmt.Errorf("marshaling signable to JSON: %w", err)
	}

	patch, err := fn()
	if err != nil {
		return nil, err
	}

	patchedJSONObject, err := patch.Apply(jsonObject)
	if err != nil {
		return nil, fmt.Errorf("patching signable for digestion: %w", err)
	}

	patchedYAMLObject, err := yaml.JSONToYAML(patchedJSONObject)
	if err != nil {
		return nil, fmt.Errorf("marshaling patched signable to YAML: %w", err)
	}

	return patchedYAMLObject, nil
}

func (s signable) excludes() ([]jsonPointer, error) {
	excludesKey := signature.DomainName + "/" + signature.ExcludesAnnotation
	b64Excludes := s.GetAnnotations()[excludesKey]
	excludes, err := base64.StdEncoding.DecodeString(b64Excludes)
	if err != nil {
		return nil, fmt.Errorf("decoding excludes annotations: %w", err)
	}

	// The excludes within the object are in JSON path format, so stick with that
	// for now, escape them when they're converted to JSON pointers.
	prefix := ".metadata.annotations."
	var pointers []jsonPointer
	for _, exclude := range strings.Split(string(excludes), ExcludesSep) {
		pointers = append(pointers, fromJSONPath(prefix+exclude))
	}

	return pointers, nil
}

// exportPatch builds a JSON patch that removes fields that shouldn't be
// exported.
func (s signable) exportPatch() (customPatch, error) {
	patch := customPatch{}
	for _, exclude := range signature.AlwaysExcluded {
		patch = append(patch, genRemovePathOp(fromJSONPath(exclude)))
	}

	return patch, nil
}

// signingPatch builds a JSON patch that removes fields that shouldn't be
// considered when generating a digest.
//
// It would be convenient to use Kustomize to manipulate the YAML k8s
// objects. However, Kustomize doesn't handle applying patches where the key to
// be removed doesn't exist. This follows the JSON Patch spec, but it's
// annoying. Fortunately, the underlying library used by Kustomize (json-patch),
// _does_ support that ability via an optional flag (a flag that Kustomize
// doesn't expose). So in order to apply patches without a ton of headache when
// they fail due to a missing key, use json-patch directly.
func (s signable) signingPatch() (customPatch, error) {
	var patch jsonpatch.Patch

	excludes, err := s.excludes()
	if err != nil {
		return nil, err
	}
	for _, exclude := range excludes {
		patch = append(patch, genRemovePathOp(exclude))
	}

	// ...plus the signature (go) package excludes...
	for _, alwaysExclude := range signature.AlwaysExcluded {
		patch = append(patch, genRemovePathOp(fromJSONPath(alwaysExclude)))
	}

	return customPatch(patch), nil
}

// genRemovePathOp returns an operation that removes a path.
func genRemovePathOp(ptr jsonPointer) jsonpatch.Operation {
	return jsonpatch.Operation{
		"op":   rawMsgPointer("remove"),
		"path": rawMsgPointer(ptr.String()),
	}
}

// customPatch wraps jsonpatch.Patch to set options for EKS-Anywhere Packages
// use.
//
// Because patchWithAllowMissingPathOnRemove is too long of a name.
type customPatch jsonpatch.Patch

// patchFunction returns a JSON patch (jsonpatch.Patch).
type patchFunction func() (customPatch, error)

// Apply applies custom options when applying.
func (p customPatch) Apply(doc []byte) ([]byte, error) {
	options := jsonpatch.NewApplyOptions()
	options.AllowMissingPathOnRemove = true
	return jsonpatch.Patch(p).ApplyWithOptions(doc, options)
}

// ApplyIndent applies custom options when applying (with indent).
func (p customPatch) ApplyIndent(doc []byte, indent string) ([]byte, error) {
	options := jsonpatch.NewApplyOptions()
	options.AllowMissingPathOnRemove = true
	return jsonpatch.Patch(p).ApplyIndentWithOptions(doc, indent, options)
}

// jsonPointer helps keep straight the format of a path (JSON path vs JSON pointer).
type jsonPointer string

func (p jsonPointer) String() string {
	return string(p)
}

// fromJSONPath converts from a JSON path to a JSON pointer.
//
// It will escape the individual path segments in the process.
func fromJSONPath(jsonPath string) jsonPointer {
	segments := strings.Split(jsonPath, ".")
	escaped := ""
	if strings.HasPrefix(jsonPath, ".") {
		escaped = "/"
	}
	for _, segment := range segments {
		escaped = path.Join(escaped, escapeStringForJSONPointer(segment))
	}

	return jsonPointer(escaped)
}

// escapeStringForJSONPointer escapes a string for use as a segment in a JSON
// pointer.
//
// https://datatracker.ietf.org/doc/html/rfc6901
func escapeStringForJSONPointer(str string) string {
	tmp := strings.ReplaceAll(str, "~", "~0")
	return strings.ReplaceAll(tmp, "/", "~1")
}

// rawMsgPointer makes it a little less painful to generate jsonpatch.Operations.
//
// Why not use jsonpatch.DecodePatch? Because it can fail. This can't fail. Which
// makes it much handier, since errors don't need to be checked.
func rawMsgPointer(s string) *json.RawMessage {
	var x json.RawMessage = json.RawMessage(fmt.Sprintf("%q", s))
	return &x
}
