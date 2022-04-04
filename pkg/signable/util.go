package signable

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
)

// Sign data with a private key.
//
// Key should be a SEC1 / ASN.1 / DER / PEM-encoded private key.
func Sign(pemKey []byte, data []byte, hash crypto.Hash) ([]byte, error) {
	blk, _ := pem.Decode(pemKey)
	var key crypto.PrivateKey
	var err error

	key, err = x509.ParseECPrivateKey(blk.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing signing key: %w", err)
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, errors.New("key isn't valid for signing")
	}

	sig, err := signer.Sign(rand.Reader, data, hash)
	if err != nil {
		return nil, fmt.Errorf("signing: %w", err)
	}

	return sig, nil
}

// SignDigest wraps sign to digest the data before signing.
//
// Key should be a SEC1 / ASN.1 / DER / PEM-encoded private key.
func SignDigest(pemKey []byte, data []byte, hash crypto.Hash) ([]byte, error) {
	h := hash.New()
	_, err := h.Write(data)
	if err != nil {
		return nil, fmt.Errorf("digesting: %w", err)
	}

	return Sign(pemKey, h.Sum(nil), hash)
}

// SignDigestBase64 wraps signDigest to base64 encode the signature.
func SignDigestBase64(pemKey []byte, data []byte, hash crypto.Hash) (
	string, error) {

	sigBytes, err := SignDigest(pemKey, data, hash)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(sigBytes), nil
}
