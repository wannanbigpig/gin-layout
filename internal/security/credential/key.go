package credential

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/wannanbigpig/gin-layout/config"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

// Purpose provides cryptographic domain separation for values derived from the
// same deployment-level credential encryption key.
type Purpose string

const (
	PurposeSMSProvider      Purpose = "sms-provider-credentials:v1"
	PurposeSMSOTP           Purpose = "sms-otp-hmac:v1"
	PurposeSMSQuota         Purpose = "sms-quota-hmac:v1"
	PurposeStorageAliyunOSS Purpose = "storage-aliyun-oss-credentials:v1"
)

const derivationDomain = "gin-layout:credential-key:v1:"

const minimumMasterKeyLength = 32

// DeriveKey derives a purpose-specific key from the deployment master key.
// The master key is never returned directly or shared across business domains.
func DeriveKey(purpose Purpose) (string, error) {
	cfg := config.GetConfig()
	if cfg == nil || strings.TrimSpace(cfg.Security.CredentialEncryptionKey) == "" {
		return "", e.NewDependencyNotReadyError("服务端未配置 security.credential_encryption_key，无法使用受保护凭据，请由运维配置后重启所有实例")
	}
	masterKey := strings.TrimSpace(cfg.Security.CredentialEncryptionKey)
	if len(masterKey) < minimumMasterKeyLength {
		return "", e.NewDependencyNotReadyError("security.credential_encryption_key 长度不足，至少需要 32 个字符；建议使用 openssl rand -base64 32 生成后重启所有实例")
	}
	mac := hmac.New(sha256.New, []byte(masterKey))
	_, _ = mac.Write([]byte(derivationDomain + string(purpose)))
	return hex.EncodeToString(mac.Sum(nil)), nil
}
