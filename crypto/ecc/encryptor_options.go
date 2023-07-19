package ecc

import (
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/utils/xconv"
)

const (
	defaultEncryptorShareInfo1Key = "config.crypto.ecc.encryptor.s1"
	defaultEncryptorShareInfo2Key = "config.crypto.ecc.encryptor.s2"
	defaultEncryptorPublicKeyKey  = "config.crypto.ecc.encryptor.publicKey"
	defaultEncryptorPrivateKeyKey = "config.crypto.ecc.encryptor.privateKey"
)

type EncryptorOption func(o *encryptorOptions)

type encryptorOptions struct {
	// 共享信息。加解密时必需一致
	// 默认为空
	s1 []byte

	// 共享信息。加解密时必需一致
	// 默认为空
	s2 []byte

	// 公钥。可设置文件路径或公钥串
	publicKey string

	// 私钥。可设置文件路径或私钥串
	privateKey string
}

func defaultEncryptorOptions() *encryptorOptions {
	return &encryptorOptions{
		s1:         config.Get(defaultEncryptorShareInfo1Key).Bytes(),
		s2:         config.Get(defaultEncryptorShareInfo2Key).Bytes(),
		publicKey:  config.Get(defaultEncryptorPublicKeyKey).String(),
		privateKey: config.Get(defaultEncryptorPrivateKeyKey).String(),
	}
}

// WithEncryptorShareInfo 设置共享信息
func WithEncryptorShareInfo(s1, s2 string) EncryptorOption {
	return func(o *encryptorOptions) { o.s1, o.s2 = xconv.StringToBytes(s1), xconv.StringToBytes(s2) }
}

// WithEncryptorPublicKey 设置加密公钥
func WithEncryptorPublicKey(publicKey string) EncryptorOption {
	return func(o *encryptorOptions) { o.publicKey = publicKey }
}

// WithEncryptorPrivateKey 设置解密私钥
func WithEncryptorPrivateKey(privateKey string) EncryptorOption {
	return func(o *encryptorOptions) { o.privateKey = privateKey }
}
