package admin_v1

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// isSSERequest 判断当前请求是否要求服务端以 SSE 方式推送处理进度。
// 客户端可通过设置 Accept: text/event-stream 或 ?stream=1 来开启。
func isSSERequest(c *gin.Context) bool {
	if c.Query("stream") == "1" {
		return true
	}
	accept := c.GetHeader("Accept")
	return strings.Contains(accept, "text/event-stream")
}

func writeSSEHeaders(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-cache, no-transform")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
}

func writeSSEEvent(w io.Writer, flusher http.Flusher, event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	if flusher != nil {
		flusher.Flush()
	}
}

func (api FileResourceController) streamUploadCredentialBatch(c *gin.Context, params *form.FileUploadCredentialBatch) {
	flusher, _ := c.Writer.(http.Flusher)
	writeSSEHeaders(c)
	writeSSEEvent(c.Writer, flusher, "start", gin.H{"total": len(params.Items)})
	progressFn := func(p service.BatchProgress) {
		writeSSEEvent(c.Writer, flusher, "progress", p)
	}
	result, err := service.NewFileResourceService().UploadCredentialBatch(params, progressFn)
	if err != nil {
		writeSSEEvent(c.Writer, flusher, "error", gin.H{"message": err.Error()})
		return
	}
	writeSSEEvent(c.Writer, flusher, "complete", result)
}

func (api FileResourceController) streamUploadCompleteBatch(c *gin.Context, params *form.FileUploadCompleteBatch, uid uint) {
	flusher, _ := c.Writer.(http.Flusher)
	writeSSEHeaders(c)
	writeSSEEvent(c.Writer, flusher, "start", gin.H{"total": len(params.Items)})
	progressFn := func(p service.BatchProgress) {
		writeSSEEvent(c.Writer, flusher, "progress", p)
	}
	result, err := service.NewFileResourceService().CompleteDirectUploadBatch(params, uid, progressFn)
	if err != nil {
		writeSSEEvent(c.Writer, flusher, "error", gin.H{"message": err.Error()})
		return
	}
	writeSSEEvent(c.Writer, flusher, "complete", result)
}
