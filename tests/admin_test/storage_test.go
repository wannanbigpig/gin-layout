package admin_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestStorageConfigAndLocalConnectivityWithConfiguredMySQL(t *testing.T) {
	requireMySQL(t)

	configResp := getRequest("/admin/v1/system/storage/config", nil)
	assert.Equal(t, http.StatusOK, configResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, configResp).Code)

	testPayload := `{
		"active_driver":"local",
		"config":{
			"local":{},
			"aliyun_oss":{},
			"signed_url_ttl_seconds":60,
			"max_file_size_mb":10
		}
	}`
	testResp := postRequest("/admin/v1/system/storage/test", &testPayload)
	assert.Equal(t, http.StatusOK, testResp.Code)
	assert.Equal(t, e.SUCCESS, decodeResult(t, testResp).Code)

	secretResp := getRequest("/admin/v1/system/storage/secret", nil)
	assert.Equal(t, http.StatusBadRequest, secretResp.Code)
	assert.Equal(t, e.InvalidParameter, decodeResult(t, secretResp).Code)
}
