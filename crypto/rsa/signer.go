package rsa

import (
	"crypto/rand"
	"crypto/rsa"

	"github.com/dobyte/due/v2/errors"
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

// Name returns the name of the signer.
func (s *Signer) Name() string {
	return Name
}

// Sign signs data with the RSA private key.
func (s *Signer) Sign(data []byte) ([]byte, error) {
	if s.privateKey == nil {
		return nil, errors.New("signer: private key is not set")
	}

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

// Verify verifies the signature of data with the RSA public key.
func (s *Signer) Verify(data []byte, signature []byte) (bool, error) {
	if s.publicKey == nil {
		return false, errors.New("signer: public key is not set")
	}

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
		err = rsa.VerifyPKCS1v15(s.publicKey, hash, hashed, signature)
	default:
		err = rsa.VerifyPSS(s.publicKey, hash, hashed, signature, &rsa.PSSOptions{
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
	if s.opts.publicKey != "" {
		s.publicKey, s.err = parsePublicKey(s.opts.publicKey)
		if s.err != nil {
			return
		}
	}

	if s.opts.privateKey != "" {
		s.privateKey, s.err = parsePrivateKey(s.opts.privateKey)
	}
}
