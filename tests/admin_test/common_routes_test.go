package admin_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestCommonUploadRequiresLogin(t *testing.T) {
	body := `{}`
	resp := anonymousPostRequest("/admin/v1/common/upload", &body)

	assert.Equal(t, http.StatusOK, resp.Code)
	result := decodeResult(t, resp)
	assert.Equal(t, e.NotLogin, result.Code)
}
