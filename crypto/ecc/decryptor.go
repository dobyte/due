package ecc

import (
	"github.com/dobyte/due/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
)

type Decryptor struct {
	err        error
	opts       *decryptorOptions
	privateKey *ecies.PrivateKey
}

var _ crypto.Decryptor = &Decryptor{}

func init() {
	crypto.RegisterDecryptor(NewDecryptor())
}

func NewDecryptor(opts ...DecryptorOption) *Decryptor {
	o := defaultDecryptorOptions()
	for _, opt := range opts {
		opt(o)
	}

	d := &Decryptor{opts: o}
	d.privateKey, d.err = parseECIESPrivateKey(d.opts.privateKey)

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
