package ecc

import (
	"github.com/dobyte/due/config"
	"github.com/dobyte/due/crypto/hash"
	"strings"
)

const (
	defaultSignerHashKey       = "config.crypto.rsa.signer.hash"
	defaultSignerDelimiterKey  = "config.crypto.rsa.signer.delimiter"
	defaultSignerPrivateKeyKey = "config.crypto.rsa.signer.privateKey"
)

type SignerOption func(o *signerOptions)

type signerOptions struct {
	// hash算法。支持sha1、sha224、sha256、sha384、sha512
	// 默认为sha256
	hash hash.Hash

	// 签名分隔符。
	delimiter string

	// 私钥。可设置文件路径或私钥串
	privateKey string
}

func defaultSignerOptions() *signerOptions {
	return &signerOptions{
		hash:       hash.Hash(strings.ToLower(config.Get(defaultSignerHashKey).String())),
		delimiter:  config.Get(defaultSignerDelimiterKey, " ").String(),
		privateKey: config.Get(defaultSignerPrivateKeyKey).String(),
	}
}

// WithSignerHash 设置加密hash算法
func WithSignerHash(hash hash.Hash) SignerOption {
	return func(o *signerOptions) { o.hash = hash }
}

// WithSignerDelimiter 设置签名分割符
func WithSignerDelimiter(delimiter string) SignerOption {
	return func(o *signerOptions) { o.delimiter = delimiter }
}

// WithSignerPrivateKey 设置解密私钥
func WithSignerPrivateKey(privateKey string) SignerOption {
	return func(o *signerOptions) { o.privateKey = privateKey }
}
