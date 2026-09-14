package admin_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestDashboardRoutesWithConfiguredMySQL(t *testing.T) {
	requireMySQL(t)

	for _, route := range []string{
		"/admin/v1/dashboard/overview",
		"/admin/v1/dashboard/statistics",
		"/admin/v1/dashboard/monitor",
	} {
		t.Run(route, func(t *testing.T) {
			resp := getRequest(route, nil)
			assert.Equal(t, http.StatusOK, resp.Code)
			result := decodeResult(t, resp)
			assert.Equal(t, e.SUCCESS, result.Code)
		})
	}
}
