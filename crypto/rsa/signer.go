package rsa

import (
	"crypto/rand"
	"crypto/rsa"
)

type Signer struct {
	err        error
	opts       *signerOptions
	privateKey *rsa.PrivateKey
}

func NewSigner(opts ...SignerOption) *Signer {
	o := defaultSignerOptions()
	for _, opt := range opts {
		opt(o)
	}

	d := &Signer{opts: o}
	d.privateKey, d.err = parsePrivateKey(d.opts.privateKey)

	return d
}

// Name 名称
func (s *Signer) Name() string {
	return Name
}

// Sign 签名
func (s *Signer) Sign(data []byte) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}

	hash := s.opts.hash.Hash()
	hashed := s.opts.hash.Sum(data)

	switch s.opts.padding {
	case PKCS:
		return rsa.SignPKCS1v15(rand.Reader, s.privateKey, hash, hashed)
	default:
		return rsa.SignPSS(rand.Reader, s.privateKey, hash, hashed, &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthEqualsHash,
			Hash:       hash,
		})
	}
}
