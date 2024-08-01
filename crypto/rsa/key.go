package rsa

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"os"
	"path"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xpath"
)

type Format int

const (
	PKCS1 Format = iota
	PKCS8
)

type Key struct {
	prv *rsa.PrivateKey
}

// GenerateKey 生成秘钥
func GenerateKey(bits int) (*Key, error) {
	prv, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, err
	}

	return &Key{prv: prv}, nil
}

// PublicKey 获取公钥
func (k *Key) PublicKey() *rsa.PublicKey {
	return &k.prv.PublicKey
}

// PrivateKey 获取私钥
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
func (k *Key) SaveKeyPair(format Format, dir string, file string) (err error) {
	err = k.savePublicKey(format, dir, file)
	if err != nil {
		return
	}

	return k.savePrivateKey(format, dir, file)
}

// 保存公钥
func (k *Key) savePrivateKey(format Format, dir string, file string) (err error) {
	filepath := path.Join(dir, file)
	defer func() {
		if err != nil {
			_ = os.Remove(filepath)
		}
	}()

	f, err := os.Create(filepath)
	if err != nil {
		return
	}

	return k.marshalPrivateKey(format, f)
}

// 保存公钥
func (k *Key) savePublicKey(format Format, dir string, file string) (err error) {
	base, _, name, ext := xpath.Split(file)
	if ext != "" {
		file = name + ".pub." + ext
	} else {
		file = name + ".pub"
	}

	filepath := path.Join(dir, base, file)
	defer func() {
		if err != nil {
			_ = os.Remove(filepath)
		}
	}()

	f, err := os.Create(filepath)
	if err != nil {
		return
	}

	return k.marshalPublicKey(format, f)
}

func loadKey(key string) (*pem.Block, error) {
	var (
		err    error
		buffer []byte
	)

	if xpath.IsFile(key) {
		buffer, err = os.ReadFile(key)
		if err != nil {
			return nil, err
		}
	} else {
		buffer = xconv.StringToBytes(key)
	}

	block, _ := pem.Decode(buffer)

	return block, nil
}

func parsePublicKey(publicKey string) (*rsa.PublicKey, error) {
	black, err := loadKey(publicKey)
	if err != nil {
		return nil, err
	}

	if black == nil {
		return nil, errors.New("invalid public key")
	}

	pkcs, err := x509.ParsePKCS1PublicKey(black.Bytes)
	if err == nil {
		return pkcs, nil
	}

	pub, err := x509.ParsePKIXPublicKey(black.Bytes)
	if err == nil {
		return pub.(*rsa.PublicKey), nil
	}

	return nil, err
}

func parsePrivateKey(privateKey string) (*rsa.PrivateKey, error) {
	black, err := loadKey(privateKey)
	if err != nil {
		return nil, err
	}

	if black == nil {
		return nil, errors.New("invalid private key")
	}

	priv, err := x509.ParsePKCS8PrivateKey(black.Bytes)
	if err == nil {
		return priv.(*rsa.PrivateKey), nil
	}

	return x509.ParsePKCS1PrivateKey(black.Bytes)
}
