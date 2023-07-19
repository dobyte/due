package rsa

import (
	"crypto/rand"
	"crypto/rsa"
)

type Signer struct {
	err        error
	opts       *signerOptions
	publicKey  *rsa.PublicKey
	privateKey *rsa.PrivateKey
}

func NewSigner(opts ...SignerOption) *Signer {
	o := defaultSignerOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Signer{opts: o}
	s.init()

	return s
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

// Verify 验签
func (s *Signer) Verify(data []byte, signature []byte) (bool, error) {
	if s.err != nil {
		return false, s.err
	}

	var (
		err    error
		hash   = s.opts.hash.Hash()
		hashed = s.opts.hash.Sum(data)
	)

	switch s.opts.padding {
	case PKCS:
		err = rsa.VerifyPKCS1v15(s.publicKey, hash, hashed[:], signature)
	default:
		err = rsa.VerifyPSS(s.publicKey, hash, hashed[:], signature, &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthEqualsHash,
			Hash:       hash,
		})
	}

	if err == rsa.ErrVerification {
		return false, nil
	}

	return err == nil, err
}

func (s *Signer) init() {
	s.publicKey, s.err = parsePublicKey(s.opts.publicKey)
	if s.err != nil {
		return
	}

	s.privateKey, s.err = parsePrivateKey(s.opts.privateKey)
}
