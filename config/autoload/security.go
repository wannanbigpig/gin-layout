package autoload

// SecurityConfig contains process-level security material. Business credentials
// remain in sys_config and are encrypted with purpose-specific derived keys.
type SecurityConfig struct {
	CredentialEncryptionKey string `mapstructure:"credential_encryption_key"`
}

var Security = SecurityConfig{}
