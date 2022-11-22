package rsa_test

import (
	"github.com/dobyte/due/crypto/hash"
	"github.com/dobyte/due/crypto/rsa"
	"github.com/dobyte/due/utils/xrand"
	"testing"
)

var (
	encryptor *rsa.Encryptor
	decryptor *rsa.Decryptor
	signer    *rsa.Signer
	verifier  *rsa.Verifier
)

const (
	rsaPublicKey  = "./pem/key.pub.pem"
	rsaPrivateKey = "./pem/key.pem"
)

func init() {
	encryptor = rsa.NewEncryptor(
		rsa.WithEncryptorHash(hash.SHA256),
		rsa.WithEncryptorPadding(rsa.OAEP),
		rsa.WithEncryptorPublicKey(rsaPublicKey),
	)

	decryptor = rsa.NewDecryptor(
		rsa.WithDecryptorHash(hash.SHA256),
		rsa.WithDecryptorPadding(rsa.OAEP),
		rsa.WithDecryptorPrivateKey(rsaPrivateKey),
	)

	signer = rsa.NewSigner(
		rsa.WithSignerHash(hash.SHA256),
		rsa.WithSignerPadding(rsa.PKCS),
		rsa.WithSignerPrivateKey(rsaPrivateKey),
	)

	verifier = rsa.NewVerifier(
		rsa.WithVerifierHash(hash.SHA256),
		rsa.WithVerifierPadding(rsa.PKCS),
		rsa.WithVerifierPublicKey(rsaPublicKey),
	)
}

func Test_Encrypt(t *testing.T) {
	str := xrand.Letters(20000)
	bytes := []byte(str)

	plaintext, err := encryptor.Encrypt(bytes)
	if err != nil {
		t.Fatal(err)
	}

	data, err := decryptor.Decrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(data) == str)
}

func Test_Sign(t *testing.T) {
	str := xrand.Letters(20000)
	bytes := []byte(str)

	signature, err := signer.Sign(bytes)
	if err != nil {
		t.Fatal(err)
	}

	ok, err := verifier.Verify(bytes, signature)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(ok)
}
