package rsa

import (
	"crypto/rsa"
)

type Verifier struct {
	err       error
	opts      *verifierOption
	publicKey *rsa.PublicKey
}

var DefaultVerifier = NewVerifier()

func NewVerifier(opts ...VerifierOption) *Verifier {
	o := defaultVerifierOptions()
	for _, opt := range opts {
		opt(o)
	}

	d := &Verifier{opts: o}
	d.publicKey, d.err = parsePublicKey(d.opts.publicKey)

	return d
}

// Name 名称
func (v *Verifier) Name() string {
	return Name
}

// Verify 验签
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
		err = rsa.VerifyPKCS1v15(v.publicKey, hash, hashed[:], signature)
	default:
		err = rsa.VerifyPSS(v.publicKey, hash, hashed[:], signature, &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthEqualsHash,
			Hash:       hash,
		})
	}

	if err == rsa.ErrVerification {
		return false, nil
	}

	return err == nil, err
}

// Verify 验签
func Verify(data []byte, signature []byte) (bool, error) {
	return DefaultVerifier.Verify(data, signature)
}
