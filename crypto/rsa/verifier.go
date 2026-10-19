package rsa

import (
	"crypto/rsa"
)

// Verifier verifies signatures with an RSA public key.
//
// It is the verification-only counterpart of Signer, so a peer that only holds the public key does
// not need to configure a private key.
type Verifier struct {
	err       error
	opts      *signerOptions
	publicKey *rsa.PublicKey
}

// NewVerifier returns a verifier built from the given public key.
//
// The public key may be a file path or a PEM-encoded key string.
func NewVerifier(publicKey string, opts ...SignerOption) *Verifier {
	o := defaultSignerOptions()
	for _, opt := range opts {
		opt(o)
	}

	v := &Verifier{opts: o}
	v.publicKey, v.err = parsePublicKey(publicKey)

	return v
}

// Verify verifies the signature of data with the RSA public key.
func (v *Verifier) Verify(data []byte, signature []byte) (bool, error) {
	if v.err != nil {
		return false, v.err
	}

	var (
		err    error
		hash   = v.opts.hash.Hash()
		hashed = v.opts.hash.Sum(data)
	)

	switch v.opts.padding {
	case PKCS:
		err = rsa.VerifyPKCS1v15(v.publicKey, hash, hashed, signature)
	default:
		err = rsa.VerifyPSS(v.publicKey, hash, hashed, signature, &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthEqualsHash,
			Hash:       hash,
		})
	}

	if err == rsa.ErrVerification {
		return false, nil
	}

	return err == nil, err
}
