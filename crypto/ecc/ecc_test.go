package ecc_test

import (
	"github.com/dobyte/due/crypto/ecc/v2"
	"github.com/dobyte/due/v2/core/hash"
	"github.com/dobyte/due/v2/utils/xrand"
	"testing"
)

const (
	publicKey  = "./pem/key.pub.pem"
	privateKey = "./pem/key.pem"
)

var (
	encryptor *ecc.Encryptor
	signer    *ecc.Signer
)

func init() {
	encryptor = ecc.NewEncryptor(
		ecc.WithEncryptorPublicKey(publicKey),
		ecc.WithEncryptorPrivateKey(privateKey),
	)

	signer = ecc.NewSigner(
		ecc.WithSignerHash(hash.SHA256),
		ecc.WithSignerPublicKey(publicKey),
		ecc.WithSignerPrivateKey(privateKey),
	)

}

func Test_Encrypt_Decrypt(t *testing.T) {
	str := xrand.Letters(200000)
	bytes := []byte(str)

	plaintext, err := encryptor.Encrypt(bytes)
	if err != nil {
		t.Fatal(err)
	}

	data, err := encryptor.Decrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(str == string(data))
}

func Benchmark_Encrypt(b *testing.B) {
	text := []byte(xrand.Letters(20000))

	for i := 0; i < b.N; i++ {
		_, err := encryptor.Encrypt(text)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_Decrypt(b *testing.B) {
	text := []byte(xrand.Letters(20000))
	plaintext, _ := encryptor.Encrypt(text)

	for i := 0; i < b.N; i++ {
		_, err := encryptor.Decrypt(plaintext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func Test_Sign_Verify(t *testing.T) {
	str := xrand.Letters(300000)
	bytes := []byte(str)

	signature, err := signer.Sign(bytes)
	if err != nil {
		t.Fatal(err)
	}

	ok, err := signer.Verify(bytes, signature)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(ok)
}

func Benchmark_Sign(b *testing.B) {
	bytes := []byte(xrand.Letters(20000))

	for i := 0; i < b.N; i++ {
		_, err := signer.Sign(bytes)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_Verify(b *testing.B) {
	bytes := []byte(xrand.Letters(20000))
	signature, _ := signer.Sign(bytes)

	for i := 0; i < b.N; i++ {
		_, err := signer.Verify(bytes, signature)
		if err != nil {
			b.Fatal(err)
		}
	}
}
