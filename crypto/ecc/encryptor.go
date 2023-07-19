package ecc

import (
	"crypto/rand"
	"github.com/ethereum/go-ethereum/crypto/ecies"
)

type Encryptor struct {
	err        error
	opts       *encryptorOptions
	publicKey  *ecies.PublicKey
	privateKey *ecies.PrivateKey
}

func NewEncryptor(opts ...EncryptorOption) *Encryptor {
	o := defaultEncryptorOptions()
	for _, opt := range opts {
		opt(o)
	}

	e := &Encryptor{opts: o}
	e.init()

	return e
}

// Name 名称
func (e *Encryptor) Name() string {
	return Name
}

// Encrypt 加密
func (e *Encryptor) Encrypt(data []byte) ([]byte, error) {
	if e.err != nil {
		return nil, e.err
	}

	return ecies.Encrypt(rand.Reader, e.publicKey, data, e.opts.s1, e.opts.s2)
}

// Decrypt 解密
func (e *Encryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	if e.err != nil {
		return nil, e.err
	}

	return e.privateKey.Decrypt(ciphertext, e.opts.s1, e.opts.s2)
}

func (e *Encryptor) init() {
	e.publicKey, e.err = parseECIESPublicKey(e.opts.publicKey)
	if e.err != nil {
		return
	}

	e.privateKey, e.err = parseECIESPrivateKey(e.opts.privateKey)
}
