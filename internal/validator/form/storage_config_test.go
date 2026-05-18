package form

import "testing"

func TestStorageConfigRejectsInvalidDriver(t *testing.T) {
	err := bindJSONBody(t, `{"active_driver":"ftp","config":{}}`, NewStorageConfigPayload())
	if err == nil {
		t.Fatal("expected invalid active_driver to fail validation")
	}
}

func TestStorageConfigAllowsLocalDriver(t *testing.T) {
	err := bindJSONBody(t, `{"active_driver":"local","config":{"signed_url_ttl_seconds":300,"max_file_size_mb":10}}`, NewStorageConfigPayload())
	if err != nil {
		t.Fatalf("expected local storage config to pass validation, got %v", err)
	}
}
