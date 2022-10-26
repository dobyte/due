package rsa

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"github.com/dobyte/due/errors"
	"github.com/dobyte/due/utils/xconv"
	"github.com/dobyte/due/utils/xpath"
	"io/ioutil"
)

type cryptor struct {
	err        error
	opts       *options
	publicKey  *rsa.PublicKey
	privateKey *rsa.PrivateKey
}

func NewCryptor(opts ...Option) *cryptor {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	c := &cryptor{opts: o}
	c.init()

	return c
}

// Encrypt 加密
func (c *cryptor) Encrypt(data []byte) ([]byte, error) {
	if c.err != nil {
		return nil, c.err
	}

	return rsa.EncryptOAEP(sha256.New(), rand.Reader, c.publicKey, data, c.opts.label)
}

// Decrypt 解密
func (c *cryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	if c.err != nil {
		return nil, c.err
	}

	return rsa.DecryptOAEP(sha256.New(), rand.Reader, c.privateKey, ciphertext, c.opts.label)
}

func (c *cryptor) init() {
	c.publicKey, c.err = c.parsePublicKey()
	if c.err != nil {
		return
	}

	c.privateKey, c.err = c.parsePrivateKey()
}

func (c *cryptor) parsePublicKey() (*rsa.PublicKey, error) {
	derBytes, err := c.loadKey(c.opts.publicKey)
	if err != nil {
		return nil, err
	}

	pub, err := x509.ParsePKIXPublicKey(derBytes)
	if err != nil {
		return nil, err
	}

	return pub.(*rsa.PublicKey), nil
}

func (c *cryptor) parsePrivateKey() (*rsa.PrivateKey, error) {
	derBytes, err := c.loadKey(c.opts.privateKey)
	if err != nil {
		return nil, err
	}
	//
	//x509.ParsePKIXPublicKey()
	//
	//x509.ParsePKIXPublicKey()

	pri, err := x509.ParsePKCS8PrivateKey(derBytes)
	if err != nil {
		return nil, err
	}

	return pri.(*rsa.PrivateKey), nil

	//return x509.ParsePKCS1PrivateKey(derBytes)
}

func (c *cryptor) loadKey(key string) ([]byte, error) {
	isFile, err := xpath.IsFile(key)
	if err != nil {
		return nil, err
	}

	var buffer []byte

	if isFile {
		buffer, err = ioutil.ReadFile(key)
		if err != nil {
			return nil, err
		}
	} else {
		buffer = xconv.StringToBytes(key)
	}

	block, _ := pem.Decode(buffer)
	if block == nil {
		return nil, errors.New("invalid public key")
	}

	return block.Bytes, nil
}
