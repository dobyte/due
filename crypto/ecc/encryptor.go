package ecc

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"github.com/dobyte/due/crypto"
	"github.com/dobyte/due/errors"
	"github.com/ethereum/go-ethereum/crypto/ecies"
)

type Encryptor struct {
	err       error
	opts      *encryptorOptions
	publicKey *ecies.PublicKey
}

func init() {
	crypto.RegistryEncryptor(NewEncryptor())
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

func (e *Encryptor) init() {
	e.publicKey, e.err = e.parsePublicKey()
	if e.err != nil {
		return
	}
}

func (e *Encryptor) parsePublicKey() (*ecies.PublicKey, error) {
	black, err := loadKey(e.opts.publicKey)
	if err != nil {
		return nil, err
	}

	if black == nil {
		return nil, errors.New("invalid public key")
	}

	pub, err := x509.ParsePKIXPublicKey(black.Bytes)
	if err == nil {
		return ecies.ImportECDSAPublic(pub.(*ecdsa.PublicKey)), nil
	}

	return nil, err
}
