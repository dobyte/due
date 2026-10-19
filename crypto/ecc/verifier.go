package ecc

import (
	"bytes"
	"crypto/ecdsa"
	"math/big"

	"github.com/dobyte/due/v2/errors"
)

// Verifier verifies signatures with an ECDSA public key.
//
// It is the verification-only counterpart of Signer, so a peer that only holds the public key does
// not need to configure a private key.
type Verifier struct {
	err       error
	opts      *signerOptions
	publicKey *ecdsa.PublicKey
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

	if o.delimiter == "" {
		v.err = errors.New("verifier: delimiter must not be empty")
		return v
	}

	v.publicKey, v.err = parseECDSAPublicKey(publicKey)

	return v
}

// Verify verifies the signature of data with the ECDSA public key.
func (v *Verifier) Verify(data []byte, signature []byte) (bool, error) {
	if v.err != nil {
		return false, v.err
	}

	segments := bytes.Split(signature, []byte(v.opts.delimiter))
	if len(segments) != 2 {
		return false, errors.ErrInvalidSignature
	}

	rs := new(big.Int)
	ss := new(big.Int)

	if err := rs.UnmarshalText(segments[0]); err != nil {
		return false, err
	}

	if err := ss.UnmarshalText(segments[1]); err != nil {
		return false, err
	}

	return ecdsa.Verify(v.publicKey, v.opts.hash.Sum(data), rs, ss), nil
}
