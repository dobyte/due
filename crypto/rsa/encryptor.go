package rsa

import (
	"crypto/rand"
	"crypto/rsa"
	"github.com/dobyte/due/v2/errors"
	"math"
)

type Encryptor struct {
	err        error
	opts       *encryptorOptions
	publicKey  *rsa.PublicKey
	privateKey *rsa.PrivateKey
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

	var (
		err       error
		black     []byte
		start     int
		end       int
		total     = int(math.Ceil(float64(len(data)) / float64(e.opts.blockSize)))
		plaintext = make([]byte, 0, total*e.publicKey.Size())
		h         = e.opts.hash.New()
	)

	for i := 0; i < total; i++ {
		start = i * e.opts.blockSize
		end = (i + 1) * e.opts.blockSize
		if end > len(data) {
			end = len(data)
		}

		switch e.opts.padding {
		case OAEP:
			black, err = rsa.EncryptOAEP(h, rand.Reader, e.publicKey, data[start:end], e.opts.label)
		default:
			black, err = rsa.EncryptPKCS1v15(rand.Reader, e.publicKey, data[start:end])
		}
		if err != nil {
			return nil, err
		}
		plaintext = append(plaintext, black...)
	}

	return plaintext, nil
}

// Decrypt 解密
func (e *Encryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	if e.err != nil {
		return nil, e.err
	}

	var (
		err   error
		black []byte
		start int
		end   int
		total = int(math.Ceil(float64(len(ciphertext)) / float64(e.privateKey.Size())))
		data  = make([]byte, 0, len(ciphertext))
		h     = e.opts.hash.New()
	)

	for i := 0; i < total; i++ {
		start = i * e.privateKey.Size()
		end = (i + 1) * e.privateKey.Size()
		if end > len(ciphertext) {
			end = len(ciphertext)
		}

		switch e.opts.padding {
		case OAEP:
			black, err = rsa.DecryptOAEP(h, rand.Reader, e.privateKey, ciphertext[start:end], e.opts.label)
		default:
			black, err = rsa.DecryptPKCS1v15(rand.Reader, e.privateKey, ciphertext[start:end])
		}
		if err != nil {
			return nil, err
		}
		data = append(data, black...)
	}

	return data, nil
}

func (e *Encryptor) init() {
	e.publicKey, e.err = parsePublicKey(e.opts.publicKey)
	if e.err != nil {
		return
	}

	e.privateKey, e.err = parsePrivateKey(e.opts.privateKey)
	if e.err != nil {
		return
	}

	var blockSize int
	switch e.opts.padding {
	case OAEP:
		blockSize = e.publicKey.Size() - 2*e.opts.hash.Size() - 2
	default:
		blockSize = e.publicKey.Size() - 11
	}

	if e.opts.blockSize <= 0 {
		e.opts.blockSize = blockSize
	} else if e.opts.blockSize > blockSize {
		e.err = errors.New("block message too long for RSA public key size")
	}
}
