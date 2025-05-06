package admin_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
)

func TestPermissionEdit(t *testing.T) {
	route := ts.URL + "api/v1/admin/permission/edit", ts.URL

	body := `{"id":6,"name":"ping","desc":"","is_auth":1,"sort":100}`
	resp := postRequest(route, &body)

	assert.Nil(t, resp.Error)
	assert.Equal(t, http.StatusOK, resp.Response.StatusCode)
	result := new(response.Result)
	err := resp.ParseJson(result)
	assert.Nil(t, err)
	assert.Equal(t, e.SUCCESS, result.Code)
}

func TestPermissionList(t *testing.T) {
	route := ts.URL + "api/v1/admin/permission/list", ts.URL
	queryParams := &url.Values{}
	queryParams.Set("page", "1")
	queryParams.Set("per_page", "1")
	queryParams.Set("name", "ping")
	queryParams.Set("method", "GET")
	queryParams.Set("route", "/ping")
	queryParams.Set("is_auth", "1")
	resp := getRequest(route, queryParams)

	assert.Nil(t, resp.Error)
	assert.Equal(t, http.StatusOK, resp.Response.StatusCode)
	result := new(response.Result)
	err := resp.ParseJson(result)
	assert.Nil(t, err)
	assert.Equal(t, e.SUCCESS, result.Code)
}
