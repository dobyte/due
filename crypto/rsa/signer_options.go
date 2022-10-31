package rsa

import (
	"strings"

	"github.com/dobyte/due/config"
)

const (
	defaultSignerHashKey    = "config.crypto.rsa.signer.hash"
	defaultSignerPaddingKey = "config.crypto.rsa.signer.padding"
)

type SignerOption func(o *signerOptions)

type signerOptions struct {
	// hash算法。支持sha1、sha224、sha256、sha384、sha512
	// 默认为sha256
	hash Hash

	// 填充规则。支持NORMAL和OAEP
	// 默认为NORMAL
	padding SignPadding

	// 私钥。可设置文件路径或私钥串
	privateKey string
}

func defaultSignerOptions() *signerOptions {
	return &signerOptions{
		hash:    Hash(strings.ToLower(config.Get(defaultSignerHashKey).String())),
		padding: SignPadding(strings.ToUpper(config.Get(defaultSignerPaddingKey).String())),
	}
}

// WithSignerHash 设置加密hash算法
func WithSignerHash(hash Hash) SignerOption {
	return func(o *signerOptions) { o.hash = hash }
}

// WithSignerPadding 设置加密填充规则
func WithSignerPadding(padding SignPadding) SignerOption {
	return func(o *signerOptions) { o.padding = padding }
}

// WithSignerPrivateKey 设置解密私钥
func WithSignerPrivateKey(privateKey string) SignerOption {
	return func(o *signerOptions) { o.privateKey = privateKey }
}
