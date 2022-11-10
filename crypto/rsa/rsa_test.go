package rsa_test

import (
	"fmt"
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

func init() {
	encryptor = rsa.NewEncryptor(
		rsa.WithEncryptorHash(hash.SHA256),
		rsa.WithEncryptorPadding(rsa.OAEP),
		rsa.WithEncryptorPublicKey("./pem/key.pem"),
	)

	decryptor = rsa.NewDecryptor(
		rsa.WithDecryptorHash(hash.SHA256),
		rsa.WithDecryptorPadding(rsa.OAEP),
		rsa.WithDecryptorPrivateKey("./pem/key.pub.pem"),
	)

	signer = rsa.NewSigner(
		rsa.WithSignerHash(hash.SHA256),
		rsa.WithSignerPadding(rsa.PKCS),
		rsa.WithSignerPrivateKey("./pem/key.pem"),
	)

	verifier = rsa.NewVerifier(
		rsa.WithVerifierHash(hash.SHA256),
		rsa.WithVerifierPadding(rsa.PKCS),
		rsa.WithVerifierPublicKey("./pem/key.pub.pem"),
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

	fmt.Println(str)
	fmt.Println(string(data))
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
