package ecc

import (
	"bytes"
	"crypto/ecdsa"
	"github.com/dobyte/due/crypto"
	"github.com/dobyte/due/errors"
	"math/big"
)

type Verifier struct {
	err       error
	opts      *verifierOption
	publicKey *ecdsa.PublicKey
}

var _ crypto.Verifier = &Verifier{}

func NewVerifier(opts ...VerifierOption) *Verifier {
	o := defaultVerifierOptions()
	for _, opt := range opts {
		opt(o)
	}

	d := &Verifier{opts: o}
	d.publicKey, d.err = parseECDSAPublicKey(d.opts.publicKey)

	return d
}

// Name 名称
func (v *Verifier) Name() string {
	return Name
}

// Verify 验签
func (v *Verifier) Verify(data []byte, signature []byte) (bool, error) {
	delimiter := []byte(v.opts.delimiter)
	segments := bytes.Split(signature, delimiter)

	if len(segments) != 2 {
		return false, errors.New("invalid signature")
	}

	rs := new(big.Int)
	ss := new(big.Int)

	if err := rs.UnmarshalText(segments[0]); err != nil {
		return false, err
	}

	if err := ss.UnmarshalText(segments[1]); err != nil {
		return false, err
	}

	hashed := v.opts.hash.Sum(data)

	return ecdsa.Verify(v.publicKey, hashed[:], rs, ss), nil
}
