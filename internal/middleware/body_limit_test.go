package middleware

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/wannanbigpig/gin-layout/config"
)

func TestRequestBodyLimitCoversKnownAndChunkedBodies(t *testing.T) {
	restore := config.UpdateConfigForTesting(func(cfg *config.Conf) { cfg.MaxRequestBodyMB = 1 })
	defer restore()
	for _, chunked := range []bool{false, true} {
		router := gin.New()
		router.Use(RequestBodyLimitHandler())
		router.POST("/payload", func(c *gin.Context) {
			_, err := io.ReadAll(c.Request.Body)
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				c.Status(http.StatusRequestEntityTooLarge)
				return
			}
			c.Status(http.StatusOK)
		})
		request := httptest.NewRequest(http.MethodPost, "/payload", bytes.NewReader(make([]byte, (1<<20)+1)))
		if chunked {
			request.ContentLength = -1
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("chunked=%v status=%d", chunked, recorder.Code)
		}
	}
}
