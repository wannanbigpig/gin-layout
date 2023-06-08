package admin_test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestLogin(t *testing.T) {
	route := fmt.Sprintf("%s/api/v1/admin/login", ts.URL)
	h := utils.HttpRequest{}

	body := `{"username": "admin","password": "123456"}`
	resp := h.JsonRequest("POST", route, strings.NewReader(body))

	assert.Nil(t, resp.Error)
	assert.Equal(t, http.StatusOK, resp.Response.StatusCode)
	result := new(response.Result)
	err := resp.ParseJson(result)
	assert.Nil(t, err)
	assert.Equal(t, e.SUCCESS, result.Code)
}

func TestGetAdminUser(t *testing.T) {
	route := fmt.Sprintf("%s/api/v1/admin-user/get", ts.URL)
	queryParams := &url.Values{}
	queryParams.Set("id", "1")
	resp := getRequest(route, queryParams)

	assert.Nil(t, resp.Error)
	assert.Equal(t, http.StatusOK, resp.Response.StatusCode)
	result := new(response.Result)
	err := resp.ParseJson(result)
	assert.Nil(t, err)
	assert.Equal(t, e.SUCCESS, result.Code)
}
