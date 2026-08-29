package xhash

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"hash/fnv"
	"io"
)

// MD5 计算 MD5 哈希值
func MD5(str string) string {
	h := md5.New()
	_, _ = io.WriteString(h, str)
	return hex.EncodeToString(h.Sum(nil))
}

// SHA256 计算 SHA-256 哈希值，key 非空时计算 HMAC-SHA256
func SHA256(data string, key string) string {
	if key != "" {
		h := hmac.New(sha256.New, []byte(key))
		h.Write([]byte(data))
		return hex.EncodeToString(h.Sum(nil))
	}

	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}

// FNV32 计算 FNV-32 哈希值
func FNV32(key string) uint32 {
	h := fnv.New32()
	h.Write([]byte(key))
	return h.Sum32()
}

// FNV32a 计算 FNV-32a 哈希值
func FNV32a(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}

// FNV64 计算 FNV-64 哈希值
func FNV64(key string) uint64 {
	h := fnv.New64()
	h.Write([]byte(key))
	return h.Sum64()
}

// FNV64a 计算 FNV-64a 哈希值
func FNV64a(key string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	return h.Sum64()
}
