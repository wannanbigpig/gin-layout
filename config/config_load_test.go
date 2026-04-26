package config

import (
	"os"
	"path/filepath"
	"testing"
)

func assertSamePath(t *testing.T, expected string, actual string) {
	t.Helper()
	expectedResolved, err := filepath.EvalSymlinks(expected)
	if err != nil {
		expectedResolved = filepath.Clean(expected)
	}
	actualResolved, err := filepath.EvalSymlinks(actual)
	if err != nil {
		actualResolved = filepath.Clean(actual)
	}
	if expectedResolved != actualResolved {
		t.Fatalf("expected %s, got %s", expectedResolved, actualResolved)
	}
}

func TestResolveConfigPathPrefersWorkingDirectoryConfig(t *testing.T) {
	t.Setenv("GO_ENV", "development")

	workDir := t.TempDir()
	configPath := filepath.Join(workDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("app:\n  port: 8080\n"), 0o644); err != nil {
		t.Fatalf("write config file failed: %v", err)
	}

	originWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originWD)
	})

	resolved, err := resolveConfigPath("")
	if err != nil {
		t.Fatalf("resolveConfigPath failed: %v", err)
	}
	assertSamePath(t, configPath, resolved)
}

func TestResolveDefaultConfigFilesUsesWorkingDirectoryExample(t *testing.T) {
	t.Setenv("GO_ENV", "development")

	workDir := t.TempDir()
	configDir := filepath.Join(workDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir failed: %v", err)
	}

	examplePath := filepath.Join(configDir, "config.yaml.example")
	if err := os.WriteFile(examplePath, []byte("app:\n  port: 8080\n"), 0o644); err != nil {
		t.Fatalf("write example file failed: %v", err)
	}

	originWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originWD)
	})

	gotExample, gotTarget, err := resolveDefaultConfigFiles()
	if err != nil {
		t.Fatalf("resolveDefaultConfigFiles failed: %v", err)
	}
	assertSamePath(t, examplePath, gotExample)
	expectedTarget := filepath.Join(filepath.Dir(filepath.Dir(gotExample)), "config.yaml")
	if gotTarget != expectedTarget {
		t.Fatalf("expected target %s, got %s", expectedTarget, gotTarget)
	}
}

func TestEnsureBasePathDefaultUsesWorkingDirectoryWhenNotConfigured(t *testing.T) {
	t.Setenv("GO_ENV", "development")

	workDir := t.TempDir()
	originWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originWD)
	})

	cfg := cloneDefaultConfig()
	cfg.BasePath = "/tmp/legacy-base-path"
	ensureBasePathDefault(cfg, false)
	assertSamePath(t, workDir, cfg.BasePath)
}

func TestEnsureBasePathDefaultKeepsExplicitConfiguredValue(t *testing.T) {
	cfg := cloneDefaultConfig()
	cfg.BasePath = "/tmp/custom-base-path"
	ensureBasePathDefault(cfg, true)
	if cfg.BasePath != "/tmp/custom-base-path" {
		t.Fatalf("expected explicit base_path to be kept, got %s", cfg.BasePath)
	}
}

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
