package resources

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestRequestLogTransformerCarriesRequestKind(t *testing.T) {
	result := NewRequestLogTransformer().ToStruct(&model.RequestLogs{
		RequestID:          "req-1",
		RequestKind:        model.RequestKindWebSocket,
		ExecutionTimeUnit:  model.ExecutionTimeUnitMilliseconds,
		ExecutionTimeScope: model.ExecutionTimeScopeConnection,
		OperationStatus:    0,
	})
	if result.RequestKind != model.RequestKindWebSocket {
		t.Fatalf("expected request_kind %q, got %q", model.RequestKindWebSocket, result.RequestKind)
	}
	if result.ExecutionTimeUnit != model.ExecutionTimeUnitMilliseconds {
		t.Fatalf("expected execution_time_unit %q, got %q", model.ExecutionTimeUnitMilliseconds, result.ExecutionTimeUnit)
	}
	if result.ExecutionTimeScope != model.ExecutionTimeScopeConnection {
		t.Fatalf("expected execution_time_scope %q, got %q", model.ExecutionTimeScopeConnection, result.ExecutionTimeScope)
	}
}
