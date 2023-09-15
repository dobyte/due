package crypto_test

import (
	"github.com/symsimmy/due/crypto/ecc"
	"github.com/symsimmy/due/crypto/rsa"
	"github.com/symsimmy/due/utils/xrand"
	"testing"
)

const (
	eccPublicKey  = "./ecc/pem/key.pub.pem"
	eccPrivateKey = "./ecc/pem/key.pem"
	rsaPublicKey  = "./rsa/pem/key.pub.pem"
	rsaPrivateKey = "./rsa/pem/key.pem"
)

var (
	eccEncryptor *ecc.Encryptor
	eccDecryptor *ecc.Decryptor
	rsaEncryptor *rsa.Encryptor
	rsaDecryptor *rsa.Decryptor
)

var (
	text         []byte
	eccPlaintext []byte
	rsaPlaintext []byte
)

func init() {
	eccEncryptor = ecc.NewEncryptor(
		ecc.WithEncryptorPublicKey(eccPublicKey),
	)
	eccDecryptor = ecc.NewDecryptor(
		ecc.WithDecryptorPrivateKey(eccPrivateKey),
	)

	rsaEncryptor = rsa.NewEncryptor(
		rsa.WithEncryptorPublicKey(rsaPublicKey),
	)

	rsaDecryptor = rsa.NewDecryptor(
		rsa.WithDecryptorPrivateKey(rsaPrivateKey),
	)

	text = []byte(xrand.Letters(20000))
	eccPlaintext, _ = eccEncryptor.Encrypt(text)
	rsaPlaintext, _ = rsaEncryptor.Encrypt(text)
}

func Benchmark_ECC_Encryptor_Encrypt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := eccEncryptor.Encrypt(text)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_RSA_Encryptor_Encrypt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := rsaEncryptor.Encrypt(text)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_ECC_Decryptor_Decrypt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := eccDecryptor.Decrypt(eccPlaintext)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_RSA_Decryptor_Decrypt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := rsaDecryptor.Decrypt(rsaPlaintext)
		if err != nil {
			b.Fatal(err)
		}
	}
}
