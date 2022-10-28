package rsa

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"github.com/dobyte/due/crypto"
	"github.com/dobyte/due/errors"
	"math"
)

type Decryptor struct {
	err        error
	opts       *decryptorOptions
	privateKey *rsa.PrivateKey
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

func (d *Decryptor) init() {
	d.privateKey, d.err = d.parsePrivateKey()
	if d.err != nil {
		return
	}
}

func (d *Decryptor) parsePrivateKey() (*rsa.PrivateKey, error) {
	black, err := loadKey(d.opts.privateKey)
	if err != nil {
		return nil, err
	}

	if black == nil {
		return nil, errors.New("invalid private key")
	}

	prv, err := x509.ParsePKCS8PrivateKey(black.Bytes)
	if err == nil {
		return prv.(*rsa.PrivateKey), nil
	}

	return x509.ParsePKCS1PrivateKey(black.Bytes)
}
