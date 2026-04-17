package admin_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestPublicDemoRoute(t *testing.T) {
	resp := anonymousGetRequest("/admin/v1/demo", nil)

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.SUCCESS, result.Code)
}

func TestPublicFileRouteWithoutAuthorization(t *testing.T) {
	resp := anonymousGetRequest("/admin/v1/file/not-found-uuid", nil)

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.NotEqual(t, e.NotLogin, result.Code)
}
