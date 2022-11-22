package ecc

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"github.com/dobyte/due/crypto"
)

type Signer struct {
	err        error
	opts       *signerOptions
	privateKey *ecdsa.PrivateKey
}

var _ crypto.Signer = &Signer{}

func init() {
	crypto.RegisterSigner(NewSigner())
}

func NewSigner(opts ...SignerOption) *Signer {
	o := defaultSignerOptions()
	for _, opt := range opts {
		opt(o)
	}

	d := &Signer{opts: o}
	d.privateKey, d.err = parseECDSAPrivateKey(d.opts.privateKey)

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

	hashed := s.opts.hash.Sum(data)

	rs, ss, err := ecdsa.Sign(rand.Reader, s.privateKey, hashed[:])
	if err != nil {
		return nil, err
	}

	rt, err := rs.MarshalText()
	if err != nil {
		return nil, err
	}

	st, err := ss.MarshalText()
	if err != nil {
		return nil, err
	}

	delimiter := []byte(s.opts.delimiter)
	buffer := &bytes.Buffer{}
	buffer.Grow(len(rt) + len(st) + len(delimiter))
	buffer.Write(rt)
	buffer.Write(delimiter)
	buffer.Write(st)

	return buffer.Bytes(), nil
}
