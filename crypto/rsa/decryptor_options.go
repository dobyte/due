package rsa

import (
	"github.com/dobyte/due/config"
	"github.com/dobyte/due/crypto/hash"
	"github.com/dobyte/due/utils/xconv"
	"strings"
)

const (
	defaultDecryptorHashKey       = "config.crypto.rsa.decryptor.hash"
	defaultDecryptorPaddingKey    = "config.crypto.rsa.decryptor.padding"
	defaultDecryptorLabelKey      = "config.crypto.rsa.decryptor.label"
	defaultDecryptorPrivateKeyKey = "config.crypto.rsa.decryptor.privateKey"
)

type DecryptorOption func(o *decryptorOptions)

type decryptorOptions struct {
	// hash算法。支持sha1、sha224、sha256、sha384、sha512
	// 默认为sha256
	hash hash.Hash

	// 填充规则。支持NORMAL和OAEP
	// 默认为NORMAL
	padding EncryptPadding

	// 标签。加解密时必需一致
	// 默认为空
	label []byte

	// 私钥。可设置文件路径或私钥串
	privateKey string
}

func defaultDecryptorOptions() *decryptorOptions {
	return &decryptorOptions{
		hash:       hash.Hash(strings.ToLower(config.Get(defaultDecryptorHashKey).String())),
		padding:    EncryptPadding(strings.ToUpper(config.Get(defaultDecryptorPaddingKey).String())),
		label:      config.Get(defaultDecryptorLabelKey).Bytes(),
		privateKey: config.Get(defaultDecryptorPrivateKeyKey).String(),
	}
}

// WithDecryptorHash 设置解密hash算法
func WithDecryptorHash(hash hash.Hash) DecryptorOption {
	return func(o *decryptorOptions) { o.hash = hash }
}

// WithDecryptorPadding 设置解密填充规则
func WithDecryptorPadding(padding EncryptPadding) DecryptorOption {
	return func(o *decryptorOptions) { o.padding = padding }
}

// WithDecryptorLabel 设置解密标签
func WithDecryptorLabel(label string) DecryptorOption {
	return func(o *decryptorOptions) { o.label = xconv.StringToBytes(label) }
}

// WithDecryptorPrivateKey 设置解密私钥
func WithDecryptorPrivateKey(privateKey string) DecryptorOption {
	return func(o *decryptorOptions) { o.privateKey = privateKey }
}
