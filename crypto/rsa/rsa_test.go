package rsa_test

import (
	"fmt"
	"github.com/dobyte/due/crypto/rsa"
	"github.com/dobyte/due/utils/xrand"
	"testing"
)

var (
	encryptor *rsa.Encryptor
	decryptor *rsa.Decryptor
)

func init() {
	encryptor = rsa.NewEncryptor(
		rsa.WithEncryptorHash(rsa.SHA256),
		rsa.WithEncryptorPadding(rsa.OAEP),
		rsa.WithEncryptorPublicKey("./pem/pkcs1_key.pub"),
	)

	decryptor = rsa.NewDecryptor(
		rsa.WithDecryptorHash(rsa.SHA256),
		rsa.WithDecryptorPadding(rsa.OAEP),
		rsa.WithDecryptorPrivateKey("./pem/pkcs1_key"),
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

func TestGenerateKey(t *testing.T) {
	key, err := rsa.GenerateKey(256)
	if err != nil {
		t.Fatal(err)
	}

	v, err := key.MarshalPublicKey(rsa.PKCS1)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(v))
}

func TestKey_SaveKeyPair(t *testing.T) {
	key, err := rsa.GenerateKey(256)
	if err != nil {
		t.Fatal(err)
	}

	err = key.SaveKeyPair(rsa.PKCS1, "./pem", "key.pem")
	if err != nil {
		t.Fatal(err)
	}
}
