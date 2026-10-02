package ecc

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"io"
	"os"
	"path"

	"github.com/dobyte/due/crypto/ecc/v2/ecies"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xos"
)

type Key struct {
	prv *ecdsa.PrivateKey
}

// GenerateKey generates a key pair on the given curve.
func GenerateKey(curve Curve) (*Key, error) {
	prv, err := ecdsa.GenerateKey(curve.New(), rand.Reader)
	if err != nil {
		return nil, err
	}

	return &Key{prv: prv}, nil
}

// PublicKey returns the public key.
func (k *Key) PublicKey() *ecdsa.PublicKey {
	return &k.prv.PublicKey
}

// PrivateKey returns the private key.
func (k *Key) PrivateKey() *ecdsa.PrivateKey {
	return k.prv
}

// MarshalPublicKey encodes the public key in PEM format.
func (k *Key) MarshalPublicKey() ([]byte, error) {
	buf := bytes.NewBuffer(nil)

	err := k.marshalPublicKey(buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// marshalPublicKey writes the public key to out in PEM format.
func (k *Key) marshalPublicKey(out io.Writer) error {
	derText, err := x509.MarshalPKIXPublicKey(k.PublicKey())
	if err != nil {
		return err
	}

	return pem.Encode(out, &pem.Block{
		Type:  "ECDSA PUBLIC KEY",
		Bytes: derText,
	})
}

// MarshalPrivateKey encodes the private key in PEM format.
func (k *Key) MarshalPrivateKey() ([]byte, error) {
	buf := bytes.NewBuffer(nil)

	err := k.marshalPrivateKey(buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// marshalPrivateKey writes the private key to out in PEM format.
func (k *Key) marshalPrivateKey(out io.Writer) error {
	derText, err := x509.MarshalECPrivateKey(k.PrivateKey())
	if err != nil {
		return err
	}

	return pem.Encode(out, &pem.Block{
		Type:  "ECDSA PRIVATE KEY",
		Bytes: derText,
	})
}

// SaveKeyPair saves the key pair under dir with the given file name.
func (k *Key) SaveKeyPair(dir string, file string) (err error) {
	if !xos.IsDir(dir) {
		err = os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			return
		}
	}

	if err = k.savePublicKey(dir, file); err != nil {
		return
	}

	if err = k.savePrivateKey(dir, file); err != nil {
		_ = os.Remove(publicKeyFilePath(dir, file))
		return
	}

	return nil
}

// savePrivateKey writes the private key to its file, removing the file on failure.
func (k *Key) savePrivateKey(dir string, file string) (err error) {
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

	return k.marshalPrivateKey(f)
}

// savePublicKey writes the public key to its file, removing the file on failure.
func (k *Key) savePublicKey(dir string, file string) (err error) {
	filepath := publicKeyFilePath(dir, file)
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

	return k.marshalPublicKey(f)
}

// publicKeyFilePath returns the file path of the public key for the given directory and file name.
// The public key file name is derived from file by inserting a ".pub" suffix before its extension.
func publicKeyFilePath(dir string, file string) string {
	base, _, name, ext := xos.Split(file)
	if ext != "" {
		file = name + ".pub." + ext
	} else {
		file = name + ".pub"
	}

	return path.Join(dir, base, file)
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

	block, _ := pem.Decode(buffer)

	return block, nil
}

func parseECIESPublicKey(publicKey string) (*ecies.PublicKey, error) {
	pub, err := parseECDSAPublicKey(publicKey)
	if err != nil {
		return nil, err
	}

	return ecies.ImportECDSAPublic(pub), nil
}

func parseECIESPrivateKey(privateKey string) (*ecies.PrivateKey, error) {
	prv, err := parseECDSAPrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	return ecies.ImportECDSA(prv), nil
}

func parseECDSAPublicKey(publicKey string) (*ecdsa.PublicKey, error) {
	block, err := loadKey(publicKey)
	if err != nil {
		return nil, err
	}

	if block == nil {
		return nil, errors.ErrInvalidPublicKey
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	switch key := pub.(type) {
	case *ecdsa.PublicKey:
		return key, nil
	default:
		return nil, errors.ErrInvalidPublicKey
	}
}

func parseECDSAPrivateKey(privateKey string) (*ecdsa.PrivateKey, error) {
	block, err := loadKey(privateKey)
	if err != nil {
		return nil, err
	}

	if block == nil {
		return nil, errors.ErrInvalidPrivateKey
	}

	return x509.ParseECPrivateKey(block.Bytes)
}
