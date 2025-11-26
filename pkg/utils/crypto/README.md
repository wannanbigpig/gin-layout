# 加密工具使用说明

本包提供 AES-256-GCM 加密算法，用于字符串的加密和解密。支持通过参数选择加密算法。

## 快速开始

```go
import "github.com/wannanbigpig/gin-layout/pkg/utils/crypto"

// 使用默认算法加密（推荐，不传算法参数）
encrypted, err := crypto.Encrypt("your-secret-key", "plaintext")
if err != nil {
    // 处理错误
}

// 使用默认算法解密
decrypted, err := crypto.Decrypt("your-secret-key", encrypted)
if err != nil {
    // 处理错误
}

// 使用指定算法加密（可选）
encrypted, err := crypto.Encrypt("your-secret-key", "plaintext", crypto.AlgorithmAES256GCM)

// 使用指定算法解密（可选）
decrypted, err := crypto.Decrypt("your-secret-key", encrypted, crypto.AlgorithmAES256GCM)
```

## 算法说明

### AES-256-GCM

**特点：**
- 使用 AES-256（高级加密标准，256 位密钥）
- GCM 模式（Galois/Counter Mode），提供认证加密（AEAD）
- 国际标准，广泛使用
- 性能优秀，硬件加速支持好
- 兼容性好，所有平台支持

**密钥处理：**
- 输入密钥为字符串，通过 SHA256 哈希派生为 32 字节密钥（AES-256 需要 32 字节）
- 每次加密使用随机 nonce（12 字节），确保相同明文产生不同密文

**密文格式：**
- 密文格式：`nonce + encrypted_data`
- 最终以 base64 编码返回

## 支持的加密算法

### AlgorithmAES256GCM

AES-256-GCM 加密算法（默认算法）

```go
crypto.AlgorithmAES256GCM
```

## API 文档

### Encrypt

```go
func Encrypt(key, plaintext string, algorithm ...Algorithm) (string, error)
```

**参数：**
- `key`: 加密密钥（字符串）
- `plaintext`: 待加密的明文
- `algorithm`: 加密算法（可选参数，可变参数，不传则使用默认算法 `AlgorithmAES256GCM`）

**返回：**
- `string`: base64 编码的密文
- `error`: 错误信息（如果算法不支持、密钥为空或加密失败）

**示例：**
```go
// 使用默认算法（推荐）
encrypted, err := crypto.Encrypt("key", "plaintext")

// 使用指定算法
encrypted, err := crypto.Encrypt("key", "plaintext", crypto.AlgorithmAES256GCM)
```

### Decrypt

```go
func Decrypt(key, ciphertext string, algorithm ...Algorithm) (string, error)
```

**参数：**
- `key`: 解密密钥（字符串，必须与加密时使用的密钥相同）
- `ciphertext`: base64 编码的密文
- `algorithm`: 解密算法（可选参数，可变参数，不传则使用默认算法 `AlgorithmAES256GCM`）

**返回：**
- `string`: 解密后的明文
- `error`: 错误信息（如果算法不支持、密钥为空、密文格式错误或解密失败）

**示例：**
```go
// 使用默认算法（推荐）
decrypted, err := crypto.Decrypt("key", encrypted)

// 使用指定算法
decrypted, err := crypto.Decrypt("key", encrypted, crypto.AlgorithmAES256GCM)
```

## 使用示例

### 示例 1：使用默认算法（推荐）

```go
package main

import (
    "fmt"
    "github.com/wannanbigpig/gin-layout/pkg/utils/crypto"
)

func main() {
    key := "my-secret-key-12345"
    plaintext := "Hello, World!"

    // 使用默认算法加密（不传算法参数）
    encrypted, err := crypto.Encrypt(key, plaintext)
    if err != nil {
        fmt.Printf("加密失败: %v\n", err)
        return
    }
    fmt.Printf("密文: %s\n", encrypted)

    // 使用默认算法解密（不传算法参数）
    decrypted, err := crypto.Decrypt(key, encrypted)
    if err != nil {
        fmt.Printf("解密失败: %v\n", err)
        return
    }
    fmt.Printf("明文: %s\n", decrypted)
}
```

### 示例 2：使用指定算法

```go
package main

import (
    "fmt"
    "github.com/wannanbigpig/gin-layout/pkg/utils/crypto"
)

func main() {
    key := "my-secret-key-12345"
    plaintext := "Hello, World!"

    // 使用指定算法加密
    encrypted, err := crypto.Encrypt(key, plaintext, crypto.AlgorithmAES256GCM)
    if err != nil {
        fmt.Printf("加密失败: %v\n", err)
        return
    }
    fmt.Printf("密文: %s\n", encrypted)

    // 使用指定算法解密
    decrypted, err := crypto.Decrypt(key, encrypted, crypto.AlgorithmAES256GCM)
    if err != nil {
        fmt.Printf("解密失败: %v\n", err)
        return
    }
    fmt.Printf("明文: %s\n", decrypted)
}
```

## 注意事项

1. **密钥管理**：请妥善保管加密密钥，建议使用环境变量或密钥管理服务
2. **密钥长度**：密钥通过 SHA256 派生为 32 字节，建议使用足够长的密钥字符串
3. **安全性**：每次加密使用随机 nonce，相同明文会产生不同密文，提高安全性
4. **错误处理**：请务必检查返回的错误，确保加密/解密操作成功
5. **空值处理**：空字符串会直接返回空字符串，不会进行加密操作

## 性能

- **加密速度**：快（有硬件加速支持）
- **解密速度**：快（有硬件加速支持）
- **CPU 占用**：低
- **内存占用**：低

## 适用场景

- 敏感数据加密存储（如 token、密码等）
- 配置文件加密
- 数据库字段加密
- 日志敏感信息加密
