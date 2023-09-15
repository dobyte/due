package ecc

import (
	"github.com/symsimmy/due/config"
	"github.com/symsimmy/due/utils/xconv"
)

const (
	defaultDecryptorShareInfo1Key = "config.crypto.ecc.decryptor.s1"
	defaultDecryptorShareInfo2Key = "config.crypto.ecc.decryptor.s2"
	defaultDecryptorPrivateKeyKey = "config.crypto.ecc.decryptor.privateKey"
)

type DecryptorOption func(o *decryptorOptions)

type decryptorOptions struct {
	// 共享信息。加解密时必需一致
	// 默认为空
	s1 []byte

	// 共享信息。加解密时必需一致
	// 默认为空
	s2 []byte

	// 私钥。可设置文件路径或私钥串
	privateKey string
}

func defaultDecryptorOptions() *decryptorOptions {
	return &decryptorOptions{
		s1:         config.Get(defaultDecryptorShareInfo1Key).Bytes(),
		s2:         config.Get(defaultDecryptorShareInfo2Key).Bytes(),
		privateKey: config.Get(defaultDecryptorPrivateKeyKey).String(),
	}
}

// WithDecryptorShareInfo 设置共享信息
func WithDecryptorShareInfo(s1, s2 string) DecryptorOption {
	return func(o *decryptorOptions) { o.s1, o.s2 = xconv.StringToBytes(s1), xconv.StringToBytes(s2) }
}

// WithDecryptorPrivateKey 设置解密私钥
func WithDecryptorPrivateKey(privateKey string) DecryptorOption {
	return func(o *decryptorOptions) { o.privateKey = privateKey }
}
