package rsa

import (
	"crypto/rand"
	"crypto/rsa"
	"github.com/dobyte/due/crypto"
	"math"
)

type Decryptor struct {
	err        error
	opts       *decryptorOptions
	privateKey *rsa.PrivateKey
}

var _ crypto.Decryptor = &Decryptor{}

func NewDecryptor(opts ...DecryptorOption) *Decryptor {
	o := defaultDecryptorOptions()
	for _, opt := range opts {
		opt(o)
	}

	d := &Decryptor{opts: o}
	d.privateKey, d.err = parsePrivateKey(d.opts.privateKey)

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

	var (
		err   error
		black []byte
		start int
		end   int
		total = int(math.Ceil(float64(len(ciphertext)) / float64(d.privateKey.Size())))
		data  = make([]byte, 0, len(ciphertext))
		h     = d.opts.hash.New()
	)

	for i := 0; i < total; i++ {
		start = i * d.privateKey.Size()
		end = (i + 1) * d.privateKey.Size()
		if end > len(ciphertext) {
			end = len(ciphertext)
		}

		switch d.opts.padding {
		case OAEP:
			black, err = rsa.DecryptOAEP(h, rand.Reader, d.privateKey, ciphertext[start:end], d.opts.label)
		default:
			black, err = rsa.DecryptPKCS1v15(rand.Reader, d.privateKey, ciphertext[start:end])
		}
		if err != nil {
			return nil, err
		}
		data = append(data, black...)
	}

	return data, nil
}
