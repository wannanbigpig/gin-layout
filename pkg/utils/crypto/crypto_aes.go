package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// AESEncrypt 使用 AES-256-GCM 加密字符串
// key: 加密密钥（字符串，会通过 SHA256 派生为 32 字节密钥）
// plaintext: 待加密的明文
// 返回: base64 编码的密文
func AESEncrypt(key, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	if key == "" {
		return "", errors.New("加密密钥不能为空")
	}

	// 从字符串密钥派生 32 字节密钥（AES-256 需要 32 字节）
	derivedKey := deriveKey256(key)

	// 创建 AES cipher
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return "", err
	}

	// 创建 GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// 生成随机 nonce（12 字节，GCM 推荐大小）
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// 加密
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// 返回 base64 编码的密文
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// AESDecrypt 使用 AES-256-GCM 解密字符串
// key: 解密密钥（字符串，会通过 SHA256 派生为 32 字节密钥）
// ciphertext: base64 编码的密文
// 返回: 解密后的明文
func AESDecrypt(key, ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	if key == "" {
		return "", errors.New("解密密钥不能为空")
	}

	// 从字符串密钥派生 32 字节密钥
	derivedKey := deriveKey256(key)

	// 解码 base64
	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	// 创建 AES cipher
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return "", err
	}

	// 创建 GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// 检查密文长度
	nonceSize := gcm.NonceSize()
	if len(ciphertextBytes) < nonceSize {
		return "", errors.New("密文长度不足")
	}

	// 提取 nonce 和密文
	nonce, ciphertextBytes := ciphertextBytes[:nonceSize], ciphertextBytes[nonceSize:]

	// 解密
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// deriveKey256 从字符串密钥派生 32 字节密钥（用于 AES-256）
func deriveKey256(key string) []byte {
	hash := sha256.Sum256([]byte(key))
	return hash[:]
}
