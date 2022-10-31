package rsa

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
)

type Signer struct {
	err        error
	opts       *signerOptions
	publicKey  *rsa.PublicKey
	privateKey *rsa.PrivateKey
}

// Sign 签名
func (s *Signer) Sign(data []byte) ([]byte, error) {
	hashed := sha256.Sum256(data)

	return rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, hashed[:])
}
