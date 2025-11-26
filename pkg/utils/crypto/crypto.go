package crypto

import "errors"

// Encrypt 使用指定算法加密字符串（默认使用 AES-256-GCM）
// key: 加密密钥（字符串，会通过 SHA256 派生为 32 字节密钥）
// plaintext: 待加密的明文
// algorithm: 加密算法（可选参数，不传则使用默认算法 AlgorithmAES256GCM）
// 返回: base64 编码的密文
func Encrypt(key, plaintext string, algorithm ...Algorithm) (string, error) {
	// 确定使用的算法
	var algo Algorithm
	if len(algorithm) > 0 && algorithm[0] != "" {
		algo = algorithm[0]
	} else {
		algo = AlgorithmAES256GCM
	}

	// 验证算法有效性
	if !algo.IsValid() {
		return "", errors.New("不支持的加密算法: " + algo.String())
	}

	// 根据算法选择加密方法
	switch algo {
	case AlgorithmAES256GCM:
		return AESEncrypt(key, plaintext)
	default:
		return "", errors.New("不支持的加密算法: " + algo.String())
	}
}

// Decrypt 使用指定算法解密字符串（默认使用 AES-256-GCM）
// key: 解密密钥（字符串，会通过 SHA256 派生为 32 字节密钥）
// ciphertext: base64 编码的密文
// algorithm: 解密算法（可选参数，不传则使用默认算法 AlgorithmAES256GCM）
// 返回: 解密后的明文
func Decrypt(key, ciphertext string, algorithm ...Algorithm) (string, error) {
	// 确定使用的算法
	var algo Algorithm
	if len(algorithm) > 0 && algorithm[0] != "" {
		algo = algorithm[0]
	} else {
		algo = AlgorithmAES256GCM
	}

	// 验证算法有效性
	if !algo.IsValid() {
		return "", errors.New("不支持的解密算法: " + algo.String())
	}

	// 根据算法选择解密方法
	switch algo {
	case AlgorithmAES256GCM:
		return AESDecrypt(key, ciphertext)
	default:
		return "", errors.New("不支持的解密算法: " + algo.String())
	}
}
