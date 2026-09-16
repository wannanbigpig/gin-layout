package admin_v1

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCommonAndAvatarUploadReportBodyLimit(t *testing.T) {
	for name, handler := range map[string]gin.HandlerFunc{"common": NewCommonController().Upload, "avatar": NewAdminUserController().UploadAvatar} {
		t.Run(name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, err := writer.CreateFormFile("file", "large.txt")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = part.Write(make([]byte, 2048)); err != nil {
				t.Fatal(err)
			}
			if err = writer.Close(); err != nil {
				t.Fatal(err)
			}
			router := gin.New()
			router.POST("/upload", func(c *gin.Context) { c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024); c.Next() }, handler)
			request := httptest.NewRequest(http.MethodPost, "/upload", &body)
			request.ContentLength = -1
			request.Header.Set("Content-Type", writer.FormDataContentType())
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("expected413, got %d: %s", response.Code, response.Body.String())
			}
		})
	}
}
