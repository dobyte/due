package ecc_test

import (
	"fmt"
	"github.com/dobyte/due/crypto/ecc"
	"github.com/dobyte/due/crypto/rsa"
	"github.com/dobyte/due/utils/xrand"
	"testing"
)

const (
	ecdsaPublicKey = `-----BEGIN ecdsa public key-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEHz/h3VLbQ3Dq8PubLVLi8cfqv2p4
i4s1LJ0HzAThWxBvQTyAF0s4VfdzLh57Lf1oNR193GdWw3w+ojA/b01Hxg==
-----END ecdsa public key-----
`
	ecdsaPrivateKey = `-----BEGIN ecdsa private key-----
MHcCAQEEIKbBaIKPAQcqA3AkMQYG5dR4ooCYitQ7rWq9lSuw670BoAoGCCqGSM49
AwEHoUQDQgAEHz/h3VLbQ3Dq8PubLVLi8cfqv2p4i4s1LJ0HzAThWxBvQTyAF0s4
VfdzLh57Lf1oNR193GdWw3w+ojA/b01Hxg==
-----END ecdsa private key-----`

	rsaPublicKey = `-----BEGIN RSA PUBLIC KEY-----
MIGJAoGBAKl547uJai8dE1tmzDGV2u/Wz7/eRDaY6BornKCrdLmElpX4eMuerR8N
5zLB2HaLTPVSgRj/Benyha806TFIrw5wGHhjnP4uSrYkgHgJPmIQxG+BzdyaM4q4
V+d9M0E23MiUlEYcb6oTxCFccyHIm1QYLaSgl2mvG3zbBheHH04bAgMBAAE=
-----END RSA PUBLIC KEY-----`

	rsaPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIICWwIBAAKBgQCpeeO7iWovHRNbZswxldrv1s+/3kQ2mOgaK5ygq3S5hJaV+HjL
nq0fDecywdh2i0z1UoEY/wXp8oWvNOkxSK8OcBh4Y5z+Lkq2JIB4CT5iEMRvgc3c
mjOKuFfnfTNBNtzIlJRGHG+qE8QhXHMhyJtUGC2koJdprxt82wYXhx9OGwIDAQAB
AoGAI3lmF909orr9UEaGO2LYvxdByTGnKZ58Bu5WDLOW7TY5pw6pikWeiz+Hw3Ib
80RZSYiJiUfoXv3qya/TmANU0ONelYBdShWfVbxiG3PViJrA4cRCXVtJCXwLQKTA
/4scZSJ/BOg6xBOy8w9QZ3Kg+JtfaLjOEzCMzwJzyD7ZrHECQQDhucXCVjvqQ4aR
5HGALGclne7SMn3Uf8oBPaVrjAcI49dZt8LWh4hAMQl+Q4rltnIyzYn/zMu37Ode
gVJ7wBW9AkEAwDTOC3vJai7o40SUjShKj5sqVgdafJQgKvo40AuyImdD4l15xQko
T+EMGy6a0sM7Kq8wrp+e7J6ZYb48BOEUtwJAcypSMKXIqexL01Gnawq8kZ+zgoEK
XAna6aknJejqiemdLQQpn0TFCmM6gbY6PptIRo1BlEZLxgpTvY7jo4nMTQJABBvc
37/4sU1paxpXNqFK3sEJaadkls8v1NtehYwKddPRTrCC3uRwOSztblNTufe0dxgh
qUn+Qr6tNrqzW8NYBwJAYcrdzbSa9eBBW7onieZAJccnFp0N4JTQO4rrxGkzRRVX
2uquFeh0BFctrg2Ys03Z5kA+hIahcY/1d0oR0kF+JQ==
-----END RSA PRIVATE KEY-----`
)

var (
	eccEncryptor *ecc.Encryptor
	eccDecryptor *ecc.Decryptor
	rsaEncryptor *rsa.Encryptor
	rsaDecryptor *rsa.Decryptor
)

var (
	text       []byte
	plaintext1 []byte
	plaintext2 []byte
)

func init() {
	eccEncryptor = ecc.NewEncryptor(
		ecc.WithEncryptorPublicKey(ecdsaPublicKey),
	)
	eccDecryptor = ecc.NewDecryptor(
		ecc.WithDecryptorPrivateKey(ecdsaPrivateKey),
	)

	rsaEncryptor = rsa.NewEncryptor(
		rsa.WithEncryptorPublicKey(rsaPublicKey),
	)

	rsaDecryptor = rsa.NewDecryptor(
		rsa.WithDecryptorPrivateKey(rsaPrivateKey),
	)

	text = []byte(xrand.Letters(20000))
	plaintext1, _ = eccEncryptor.Encrypt(text)
	plaintext2, _ = rsaEncryptor.Encrypt(text)
}

func Test_Encrypt(t *testing.T) {
	str := xrand.Letters(20000)
	bytes := []byte(str)

	plaintext, err := eccEncryptor.Encrypt(bytes)
	if err != nil {
		t.Fatal(err)
	}

	data, err := eccDecryptor.Decrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(str)
	fmt.Println(string(data))
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
		_, err := eccDecryptor.Decrypt(plaintext1)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_RSA_Decryptor_Decrypt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := rsaDecryptor.Decrypt(plaintext2)
		if err != nil {
			b.Fatal(err)
		}
	}
}
