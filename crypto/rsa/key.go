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
	"github.com/dobyte/due/v2/utils/xos"
)

type Format int

const (
	PKCS1 Format = iota
	PKCS8
)

type Key struct {
	prv *rsa.PrivateKey
}

// GenerateKey 生成密钥
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
	var (
		derText   []byte
		blockType string
	)
	switch format {
	case PKCS1:
		derText = x509.MarshalPKCS1PublicKey(k.PublicKey())
		blockType = "RSA PUBLIC KEY"
	case PKCS8:
		derText, err = x509.MarshalPKIXPublicKey(k.PublicKey())
		if err != nil {
			return
		}
		blockType = "PUBLIC KEY"
	default:
		return errors.New("invalid key format")
	}

	err = pem.Encode(out, &pem.Block{
		Type:  blockType,
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
	var (
		derText   []byte
		blockType string
	)
	switch format {
	case PKCS1:
		derText = x509.MarshalPKCS1PrivateKey(k.PrivateKey())
		blockType = "RSA PRIVATE KEY"
	case PKCS8:
		derText, err = x509.MarshalPKCS8PrivateKey(k.PrivateKey())
		if err != nil {
			return
		}
		blockType = "PRIVATE KEY"
	default:
		return errors.New("invalid key format")
	}

	err = pem.Encode(out, &pem.Block{
		Type:  blockType,
		Bytes: derText,
	})

	return
}

// SaveKeyPair 保存密钥对
func (k *Key) SaveKeyPair(format Format, dir string, file string) (err error) {
	err = k.savePublicKey(format, dir, file)
	if err != nil {
		return
	}

	return k.savePrivateKey(format, dir, file)
}

// 保存私钥
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
	defer f.Close()

	return k.marshalPrivateKey(format, f)
}

// 保存公钥
func (k *Key) savePublicKey(format Format, dir string, file string) (err error) {
	subdir, _, name, ext := xos.Split(file)
	if ext != "" {
		file = name + ".pub." + ext
	} else {
		file = name + ".pub"
	}

	filepath := path.Join(dir, subdir, file)
	defer func() {
		if err != nil {
			_ = os.Remove(filepath)
		}
	}()

	f, err := os.Create(filepath)
	if err != nil {
		return
	}
	defer f.Close()

	return k.marshalPublicKey(format, f)
}

func loadKey(key string) (*pem.Block, error) {
	var (
		err    error
		buffer []byte
	)

	if xos.IsFile(key) {
		buffer, err = os.ReadFile(key)
		if err != nil {
			return nil, err
		}
	} else {
		buffer = xconv.StringToBytes(key)
	}

	block, rest := pem.Decode(buffer)
	if block != nil && len(bytes.TrimSpace(rest)) > 0 {
		return nil, errors.ErrInvalidFormat
	}

	return block, nil
}

func parsePublicKey(publicKey string) (*rsa.PublicKey, error) {
	block, err := loadKey(publicKey)
	if err != nil {
		return nil, err
	}

	if block == nil {
		return nil, errors.ErrInvalidPublicKey
	}

	pkcs, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err == nil {
		return pkcs, nil
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err == nil {
		if key, ok := pub.(*rsa.PublicKey); ok {
			return key, nil
		}

		return nil, errors.ErrInvalidPublicKey
	}

	return nil, err
}

func parsePrivateKey(privateKey string) (*rsa.PrivateKey, error) {
	block, err := loadKey(privateKey)
	if err != nil {
		return nil, err
	}

	if block == nil {
		return nil, errors.ErrInvalidPrivateKey
	}

	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		if key, ok := priv.(*rsa.PrivateKey); ok {
			return key, nil
		}

		return nil, errors.ErrInvalidPrivateKey
	}

	return x509.ParsePKCS1PrivateKey(block.Bytes)
}
