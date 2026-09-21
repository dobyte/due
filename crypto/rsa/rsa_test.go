package rsa_test

import (
	"testing"

	"github.com/dobyte/due/crypto/rsa/v2"
	"github.com/dobyte/due/v2/core/hash"
	"github.com/dobyte/due/v2/utils/xrand"
)

var (
	encryptor *rsa.Encryptor
	signer    *rsa.Signer
)

const (
	publicKey  = "./pem/key.pub.pem"
	privateKey = "./pem/key.pem"
)

func init() {
	encryptor = rsa.NewEncryptor(
		rsa.WithEncryptorHash(hash.SHA256),
		rsa.WithEncryptorPadding(rsa.OAEP),
		rsa.WithEncryptorPublicKey(publicKey),
		rsa.WithEncryptorPrivateKey(privateKey),
	)

	signer = rsa.NewSigner(
		rsa.WithSignerHash(hash.SHA256),
		rsa.WithSignerPadding(rsa.PKCS),
		rsa.WithSignerPublicKey(publicKey),
		rsa.WithSignerPrivateKey(privateKey),
	)
}

func Test_Encrypt_Decrypt(t *testing.T) {
	str := xrand.Letters(20000)
	bytes := []byte(str)

	plaintext, err := encryptor.Encrypt(bytes)
	if err != nil {
		t.Fatal(err)
	}

	data, err := encryptor.Decrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != str {
		t.Fatal("decrypt result mismatch")
	}
}

func Benchmark_Encrypt(b *testing.B) {
	text := []byte(xrand.Letters(20000))

	for b.Loop() {
		_, err := encryptor.Encrypt(text)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_Decrypt(b *testing.B) {
	text := []byte(xrand.Letters(20000))
	plaintext, _ := encryptor.Encrypt(text)

	for b.Loop() {
		_, err := encryptor.Decrypt(plaintext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func Test_Sign_Verify(t *testing.T) {
	str := xrand.Letters(20000)
	bytes := []byte(str)

	signature, err := signer.Sign(bytes)
	if err != nil {
		t.Fatal(err)
	}

	ok, err := signer.Verify(bytes, signature)
	if err != nil {
		t.Fatal(err)
	}

	if !ok {
		t.Fatal("signature verification failed")
	}
}

func Benchmark_Sign(b *testing.B) {
	bytes := []byte(xrand.Letters(20000))

	for b.Loop() {
		_, err := signer.Sign(bytes)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_Verify(b *testing.B) {
	bytes := []byte(xrand.Letters(20000))
	signature, _ := signer.Sign(bytes)

	for b.Loop() {
		_, err := signer.Verify(bytes, signature)
		if err != nil {
			b.Fatal(err)
		}
	}
}
