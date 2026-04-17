package config

import "testing"

func TestValidateJWTSecretKeyRejectsNilConfig(t *testing.T) {
	if err := validateJWTSecretKey(nil); err == nil {
		t.Fatal("expected nil config to return error")
	}
}

func TestCheckJwtSecretKeyRejectsEmptySecret(t *testing.T) {
	original := GetConfig()
	testCfg := cloneDefaultConfig()
	testCfg.Jwt.SecretKey = ""
	setActiveConfig(testCfg)
	t.Cleanup(func() { setActiveConfig(original) })

	if err := checkJwtSecretKey(); err == nil {
		t.Fatal("expected empty jwt secret key to return error")
	}
}

func TestCheckJwtSecretKeyRejectsWeakProdSecret(t *testing.T) {
	original := GetConfig()
	testCfg := cloneDefaultConfig()
	testCfg.AppEnv = "prod"
	testCfg.Jwt.SecretKey = "default-secret-key"
	setActiveConfig(testCfg)
	t.Cleanup(func() { setActiveConfig(original) })

	if err := checkJwtSecretKey(); err == nil {
		t.Fatal("expected weak prod jwt secret key to return error")
	}
}

func TestCheckJwtSecretKeyAllowsLocalWeakSecret(t *testing.T) {
	original := GetConfig()
	testCfg := cloneDefaultConfig()
	testCfg.AppEnv = "local"
	testCfg.Jwt.SecretKey = "default-secret-key"
	setActiveConfig(testCfg)
	t.Cleanup(func() { setActiveConfig(original) })

	if err := checkJwtSecretKey(); err != nil {
		t.Fatalf("expected local weak secret key to pass, got %v", err)
	}
}
