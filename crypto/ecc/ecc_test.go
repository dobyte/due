package ecc_test

import (
	"fmt"
	"github.com/symsimmy/due/crypto/ecc"
	"github.com/symsimmy/due/crypto/hash"
	"github.com/symsimmy/due/utils/xrand"
	"testing"
)

const (
	eccPublicKey  = "./pem/key.pub.pem"
	eccPrivateKey = "./pem/key.pem"
)

var (
	encryptor *ecc.Encryptor
	decryptor *ecc.Decryptor
	signer    *ecc.Signer
	verifier  *ecc.Verifier
)

func init() {
	encryptor = ecc.NewEncryptor(
		ecc.WithEncryptorPublicKey(eccPublicKey),
	)
	decryptor = ecc.NewDecryptor(
		ecc.WithDecryptorPrivateKey(eccPrivateKey),
	)

	signer = ecc.NewSigner(
		ecc.WithSignerHash(hash.SHA256),
		ecc.WithSignerPrivateKey(eccPrivateKey),
	)

	verifier = ecc.NewVerifier(
		ecc.WithVerifierHash(hash.SHA256),
		ecc.WithVerifierPublicKey(eccPublicKey),
	)
}

func Test_Encrypt(t *testing.T) {
	str := xrand.Letters(200000)
	bytes := []byte(str)

	plaintext, err := encryptor.Encrypt(bytes)
	if err != nil {
		t.Fatal(err)
	}

	data, err := decryptor.Decrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(str == string(data))
}

func Test_Sign(t *testing.T) {
	str := xrand.Letters(300000)
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
