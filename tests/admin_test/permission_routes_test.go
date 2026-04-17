package admin_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestPermissionEditRequiresLogin(t *testing.T) {
	body := `{"id":10,"name":"ping","description":"服务心跳检测接口","method":"GET","route":"/ping","is_auth":0,"sort":100}`
	resp := anonymousPostRequest("/admin/v1/permission/update", &body)

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.NotLogin, result.Code)
}

func TestPermissionListRequiresLogin(t *testing.T) {
	queryParams := &url.Values{}
	queryParams.Set("page", "1")
	queryParams.Set("per_page", "1")
	queryParams.Set("name", "ping")
	queryParams.Set("method", "GET")
	queryParams.Set("route", "/ping")
	queryParams.Set("is_auth", "1")
	resp := anonymousGetRequest("/admin/v1/permission/list", queryParams)

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.NotLogin, result.Code)
}

func TestPermissionListWithAuthorization(t *testing.T) {
	requireMySQL(t)

	queryParams := &url.Values{}
	queryParams.Set("page", "1")
	queryParams.Set("per_page", "5")
	queryParams.Set("method", "GET")

	resp := getRequest("/admin/v1/permission/list", queryParams)

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.SUCCESS, result.Code)
}

func TestPermissionUpdateValidationWithAuthorization(t *testing.T) {
	requireMySQL(t)

	body := `{}`
	resp := postRequest("/admin/v1/permission/update", &body)

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.InvalidParameter, result.Code)
}
