package resources

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestRequestLogTransformerCarriesRequestKind(t *testing.T) {
	result := NewRequestLogTransformer("zh-CN").ToStruct(&model.RequestLogs{
		RequestID:          "req-1",
		RequestKind:        model.RequestKindWebSocket,
		ExecutionTimeUs:    21110000,
		ExecutionTimeScope: model.ExecutionTimeScopeConnection,
		OperationStatus:    0,
	})
	if result.RequestKind != model.RequestKindWebSocket {
		t.Fatalf("expected request_kind %q, got %q", model.RequestKindWebSocket, result.RequestKind)
	}
	if result.ExecutionTimeUs != 21110000 {
		t.Fatalf("expected execution_time_us %d, got %d", 21110000, result.ExecutionTimeUs)
	}
	if result.ExecutionTimeScope != model.ExecutionTimeScopeConnection {
		t.Fatalf("expected execution_time_scope %q, got %q", model.ExecutionTimeScopeConnection, result.ExecutionTimeScope)
	}
}
