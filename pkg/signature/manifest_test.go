package signature_test

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/eks-anywhere-packages/pkg/signature"
	"github.com/aws/eks-anywhere-packages/pkg/testutil"
)

const (
	TestPublicKey = "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEvME/v61IfA4ulmgdF10Ae/WCRqtXvrUtF+0nu0dbdP36u3He4GRepYdQGCmbPe0463yAABZs01/Vv/v52ktlmg=="
)

var (
	EksaDomain          = Domain{Name: "eksa.aws.com", Pubkey: TestPublicKey}
	ExcludesAnnotation  = signature.ExcludesAnnotation
	SignatureAnnotation = signature.SignatureAnnotation
)

type Domain = signature.Domain

var (
	ValidateSignature      = signature.ValidateSignature
	GetDigest              = signature.GetDigest
	GetMetadataInformation = signature.GetMetadataInformation
)

func encodedSelectors(selectors []string) (encoded string) {
	return base64.StdEncoding.EncodeToString([]byte(strings.Join(selectors, "\n")))
}

func TestValidateSignature(t *testing.T) {
	t.Run("valid signature on valid manifest", func(t *testing.T) {
		bundle, _, err := testutil.GivenPackageBundle("testdata/packagebundle_valid.yaml")
		if err != nil {
			t.Fatal("Unable to get bundle", err)
		}
		valid, _, _, err := ValidateSignature(bundle, EksaDomain)

		assert.True(t, valid, "Signature should be valid for the EKS-A domain")
		assert.Nilf(t, err, "An error occured validating the signature: %v", err)
	})

	t.Run("invalid signature on valid manifest", func(t *testing.T) {
		bundle, _, _ := testutil.GivenPackageBundle("testdata/packagebundle_valid.yaml")
		annotations := bundle.GetAnnotations()
		annotations[EksaDomain.Name+"/"+SignatureAnnotation] = "XEQCIForAHp6tPUhkfqLfAzbmq0v7p/hgJEqrB5ScWNB4rOOAiBlNtJUzTWNKGxpTepnm8co0YzoNX2HjXRTvaBYQy54Tg=="
		valid, _, _, err := ValidateSignature(bundle, EksaDomain)

		assert.False(t, valid, "Signature should be invalid for the EKS-A domain")
		assert.Nilf(t, err, "An error occured validating the signature: %v", err)
	})

	t.Run("request for different domain, valid eksa signature, missing requested signature", func(t *testing.T) {
		bundle, _, _ := testutil.GivenPackageBundle("testdata/packagebundle_valid.yaml")
		valid, _, _, err := ValidateSignature(bundle, Domain{Name: "eksx.amazon.com", Pubkey: "notakey"})

		assert.False(t, valid, "Signature should be invalid for the provided domain")
		assert.EqualError(t, err, "Missing signature")
	})

	t.Run("request for different domain, missing eksa signature, requested signature invalid", func(t *testing.T) {
		bundle, _, _ := testutil.GivenPackageBundle("testdata/packagebundle_valid.yaml")
		annotations := bundle.GetAnnotations()
		domain := Domain{Name: "fakedomain.com", Pubkey: EksaDomain.Pubkey}
		annotations[domain.Name+"/signature"] = annotations[EksaDomain.Name+"/signature"]
		delete(annotations, EksaDomain.Name+"/signature")
		annotations[domain.Name+"/excludes"] = annotations[EksaDomain.Name+"/excludes"]
		delete(annotations, EksaDomain.Name+"/excludes")

		valid, _, _, err := ValidateSignature(bundle, domain)
		assert.False(t, valid, "Signature should be invalid for the provided domain because `excludes` was changed to a different domain")
		assert.Nil(t, err, "No error should occur when validating this signature")

		valid, _, _, err = ValidateSignature(bundle, EksaDomain)
		assert.False(t, valid, "Signature should be invalid for the Eksa domain")
		assert.EqualError(t, err, "Missing signature")
	})
	t.Run("Valid document with all fields excluded must fail signature validation", func(t *testing.T) {
		bundle, _, _ := testutil.GivenPackageBundle("testdata/packagebundle_valid.yaml")
		annotations := bundle.GetAnnotations()
		excludes := []string{".apiVersion", ".kind", ".metadata", ".spec"}
		annotations[EksaDomain.Name+"/excludes"] = encodedSelectors(excludes)

		digest, _, err := GetDigest(bundle, EksaDomain)

		// An empty object serializes to "{}" in yaml.
		assert.Equal(t, sha256.Sum256([]byte("{}\n")), digest, "This tests validates the behavior of an affectively empty document signature made empty via excludes")
		assert.Nil(t, err)
		valid, _, _, err := ValidateSignature(bundle, EksaDomain)
		assert.False(t, valid, "Signature should be invalid as it's signing actual content")
		assert.Nil(t, err, "No error should occur when validating this signature")
	})
	t.Run("Removing the signature causes validation to fail", func(t *testing.T) {
		bundle, _, _ := testutil.GivenPackageBundle("testdata/packagebundle_valid.yaml")
		annotations := bundle.GetAnnotations()
		delete(annotations, EksaDomain.Name+"/signature")

		valid, _, _, err := ValidateSignature(bundle, EksaDomain)
		assert.False(t, valid, "Signature should be invalid as it is missing")
		assert.EqualError(t, err, "Missing signature")
	})

	t.Run("Invalid signature format causes validation to fail with an helpful message", func(t *testing.T) {
		bundle, _, _ := testutil.GivenPackageBundle("testdata/packagebundle_valid.yaml")
		annotations := bundle.GetAnnotations()
		annotations[EksaDomain.Name+"/signature"] = annotations[EksaDomain.Name+"/signature"] + "="

		valid, _, _, err := ValidateSignature(bundle, EksaDomain)
		assert.False(t, valid, "Signature should be invalid as it is missing")
		assert.EqualError(t, err, "Signature in metadata isn't base64 encoded")
	})

	t.Run("An otherwise valid signature invalid for the provided excludes fails verification", func(t *testing.T) {
		bundle, _, _ := testutil.GivenPackageBundle("testdata/packagebundle_valid.yaml")
		annotations := bundle.GetAnnotations()
		excludes := []string{".spec.packages[].source.repository", ".spec.packages[].source.registry", ".spec.packages[].source.name"}
		annotations[EksaDomain.Name+"/excludes"] = encodedSelectors(excludes)

		valid, _, _, err := ValidateSignature(bundle, EksaDomain)
		assert.False(t, valid, "Signature should be invalid as it is missing")
		assert.Nil(t, err, "No error, the signature is simply invalid")
	})

	t.Run("Any modification to excludes, even no-op renders the signature invalid", func(t *testing.T) {
		bundle, _, _ := testutil.GivenPackageBundle("testdata/packagebundle_valid.yaml")
		annotations := bundle.GetAnnotations()
		excludes := []string{".spec.packages[].source.repository", ".spec.packages[].source.registry", ".potato"}
		annotations[EksaDomain.Name+"/excludes"] = encodedSelectors(excludes)

		valid, _, _, err := ValidateSignature(bundle, EksaDomain)
		assert.False(t, valid, "Signature is invalid, .potato was added to excludes, invalidating the signature")
		assert.Nil(t, err, "No error, the signature is simply invalid")
	})

	t.Run("A pod could also be signed", func(t *testing.T) {
		pod, _, _ := testutil.GivenPod("testdata/pod_valid.yaml")
		valid, _, _, err := ValidateSignature(pod, EksaDomain)
		assert.True(t, valid, "Signature should be valid")
		assert.Nil(t, err, "No error, the signature should be valid")
	})
}

func TestMetadata(t *testing.T) {
	t.Run("Basic metadata on valid manifest", func(t *testing.T) {
		bundle, _, _ := testutil.GivenPackageBundle("testdata/packagebundle_valid.yaml")
		sig, excludes, _ := GetMetadataInformation(bundle, EksaDomain)

		assert.ElementsMatchf(t, excludes, []string{".spec.packages[].source.registry", ".spec.packages[].source.repository"}, "Excludes doesn't match the expected value")
		assert.IsType(t, "", sig)
		assert.Len(t, sig, 96, "Signature length is wrong")
	})

	t.Run("Invalid excludes fails", func(t *testing.T) {
		bundle, _, _ := testutil.GivenPackageBundle("testdata/packagebundle_valid.yaml")
		annotations := bundle.GetAnnotations()
		annotations[EksaDomain.Name+"/"+ExcludesAnnotation] = encodedSelectors([]string{"invalid"})
		_, excludes, err := GetMetadataInformation(bundle, EksaDomain)

		assert.EqualError(t, err, "Invalid selector(s) provided")
		assert.Nil(t, excludes)

		annotations[EksaDomain.Name+"/"+ExcludesAnnotation] = annotations[EksaDomain.Name+"/"+ExcludesAnnotation] + "AA" // Invalid b64
		_, excludes, err = GetMetadataInformation(bundle, EksaDomain)

		var b64Error base64.CorruptInputError = 12
		assert.ErrorIs(t, err, b64Error, "Wrong error type")
		assert.Nil(t, excludes)
	})

	t.Run("jq imports fail validation", func(t *testing.T) {
		bundle, _, _ := testutil.GivenPackageBundle("testdata/packagebundle_valid.yaml")
		annotations := bundle.GetAnnotations()
		annotations[EksaDomain.Name+"/"+ExcludesAnnotation] = encodedSelectors([]string{"import \"test\""})
		_, excludes, err := GetMetadataInformation(bundle, EksaDomain)

		assert.NotNil(t, err)
		assert.Nil(t, excludes)
	})
}

func TestDigest(t *testing.T) {
	t.Run("Basic digest on valid manifest", func(t *testing.T) {
		bundle, expectedDigest, err := testutil.GivenPackageBundle("testdata/packagebundle_valid.yaml")
		assert.NoError(t, err)
		digest, _, err := GetDigest(bundle, EksaDomain)
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, expectedDigest, base64.StdEncoding.EncodeToString(digest[:]), "Digest doesn't match")
	})

	t.Run("Basic digest on valid manifest with no excludes", func(t *testing.T) {
		bundle, expectedDigest, err := testutil.GivenPod("testdata/pod_valid.yaml")
		assert.NoError(t, err)
		annotations := bundle.GetAnnotations()
		delete(annotations, EksaDomain.Name+"/excludes")
		digest, _, err := GetDigest(bundle, EksaDomain)
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, expectedDigest, base64.StdEncoding.EncodeToString(digest[:]), "Digest doesn't match")
	})

	t.Run("Getting digest on manifest with invalid metadata fails", func(t *testing.T) {
		bundle, _, _ := testutil.GivenPackageBundle("testdata/packagebundle_valid.yaml")
		annotations := bundle.GetAnnotations()
		annotations[EksaDomain.Name+"/"+ExcludesAnnotation] = "X"
		digest, _, err := GetDigest(bundle, EksaDomain)
		var b64error base64.CorruptInputError = 0
		assert.ErrorIs(t, err, b64error)
		assert.Equal(t, digest, [32]byte{})
	})
}
