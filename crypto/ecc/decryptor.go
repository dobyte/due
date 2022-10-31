package ecc

import (
	"crypto/x509"
	"github.com/dobyte/due/crypto"
	"github.com/dobyte/due/errors"
	"github.com/ethereum/go-ethereum/crypto/ecies"
)

type Decryptor struct {
	err        error
	opts       *decryptorOptions
	privateKey *ecies.PrivateKey
}

func init() {
	crypto.RegistryDecryptor(NewDecryptor())
}

func NewDecryptor(opts ...DecryptorOption) *Decryptor {
	o := defaultDecryptorOptions()
	for _, opt := range opts {
		opt(o)
	}

	d := &Decryptor{opts: o}
	d.init()

	return d
}

// Name 名称
func (d *Decryptor) Name() string {
	return Name
}

// Decrypt 解密
func (d *Decryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	if d.err != nil {
		return nil, d.err
	}

	return d.privateKey.Decrypt(ciphertext, d.opts.s1, d.opts.s2)
}

func (d *Decryptor) init() {
	d.privateKey, d.err = d.parsePrivateKey()
	if d.err != nil {
		return
	}
}

func (d *Decryptor) parsePrivateKey() (*ecies.PrivateKey, error) {
	black, err := loadKey(d.opts.privateKey)
	if err != nil {
		return nil, err
	}

	if black == nil {
		return nil, errors.New("invalid private key")
	}

	prv, err := x509.ParseECPrivateKey(black.Bytes)
	if err != nil {
		return nil, err
	}

	return ecies.ImportECDSA(prv), nil
}
