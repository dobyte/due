package rsa

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"github.com/dobyte/due/errors"
	"io"
	"os"
	"path"
)

type Format int

const (
	PKCS1 Format = iota
	PKCS8
)

type Key struct {
	prv *rsa.PrivateKey
}

func GenerateKey(bits int) (*Key, error) {
	prv, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, err
	}

	return &Key{prv: prv}, nil
}

func (k *Key) PublicKey() *rsa.PublicKey {
	return &k.prv.PublicKey
}

func (k *Key) PrivateKey() *rsa.PrivateKey {
	return k.prv
}

// MarshalPublicKey 编码公钥
func (k *Key) MarshalPublicKey(format Format) ([]byte, error) {
	buf := bytes.NewBuffer(nil)

	err := k.marshalPublicKey(format, buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// 编码公钥
func (k *Key) marshalPublicKey(format Format, out io.Writer) (err error) {
	var derText []byte
	switch format {
	case PKCS1:
		derText = x509.MarshalPKCS1PublicKey(k.PublicKey())
	case PKCS8:
		derText, err = x509.MarshalPKIXPublicKey(k.PublicKey())
		if err != nil {
			return
		}
	default:
		return errors.New("invalid key format")
	}

	err = pem.Encode(out, &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: derText,
	})

	return
}

// MarshalPrivateKey 编码私钥
func (k *Key) MarshalPrivateKey(format Format) ([]byte, error) {
	buf := bytes.NewBuffer(nil)

	err := k.marshalPrivateKey(format, buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// 编码私钥
func (k *Key) marshalPrivateKey(format Format, out io.Writer) (err error) {
	var derText []byte
	switch format {
	case PKCS1:
		derText = x509.MarshalPKCS1PrivateKey(k.PrivateKey())
	case PKCS8:
		derText, err = x509.MarshalPKCS8PrivateKey(k.PrivateKey())
		if err != nil {
			return
		}
	default:
		return errors.New("invalid key format")
	}

	err = pem.Encode(out, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: derText,
	})

	return
}

// SaveKeyPair 保存秘钥对
func (k *Key) SaveKeyPair(format Format, dir string, filename string) (err error) {
	err = k.savePublicKey(format, dir, filename)
	if err != nil {
		return
	}

	return k.savePrivateKey(format, dir, filename)
}

// 保存公钥
func (k *Key) savePrivateKey(format Format, dir string, filename string) (err error) {
	filepath := path.Join(dir, filename)
	defer func() {
		if err != nil {
			_ = os.Remove(filepath)
		}
	}()

	file, err := os.Create(filepath)
	if err != nil {
		return
	}

	return k.marshalPrivateKey(format, file)
}

// 保存公钥
func (k *Key) savePublicKey(format Format, dir string, filename string) (err error) {
	filepath := path.Join(dir, filename+".pub")
	defer func() {
		if err != nil {
			_ = os.Remove(filepath)
		}
	}()

	file, err := os.Create(filepath)
	if err != nil {
		return
	}

	return k.marshalPublicKey(format, file)
}
