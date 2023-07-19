package ecc

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"github.com/dobyte/due/v2/errors"
	"math/big"
)

type Signer struct {
	err        error
	opts       *signerOptions
	publicKey  *ecdsa.PublicKey
	privateKey *ecdsa.PrivateKey
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

// Verify 验签
func (s *Signer) Verify(data []byte, signature []byte) (bool, error) {
	delimiter := []byte(s.opts.delimiter)
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

	hashed := s.opts.hash.Sum(data)

	return ecdsa.Verify(s.publicKey, hashed[:], rs, ss), nil
}

func (s *Signer) init() {
	s.publicKey, s.err = parseECDSAPublicKey(s.opts.publicKey)
	if s.err != nil {
		return
	}

	s.privateKey, s.err = parseECDSAPrivateKey(s.opts.privateKey)
}
