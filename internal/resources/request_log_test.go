package resources

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestRequestLogTransformerCarriesRequestKind(t *testing.T) {
	result := NewRequestLogTransformer().ToStruct(&model.RequestLogs{
		RequestID:          "req-1",
		RequestKind:        model.RequestKindSSE,
		ExecutionTimeUnit:  model.ExecutionTimeUnitMilliseconds,
		ExecutionTimeScope: model.ExecutionTimeScopeStream,
		OperationStatus:    0,
	})
	if result.RequestKind != model.RequestKindSSE {
		t.Fatalf("expected request_kind %q, got %q", model.RequestKindSSE, result.RequestKind)
	}
	if result.ExecutionTimeUnit != model.ExecutionTimeUnitMilliseconds {
		t.Fatalf("expected execution_time_unit %q, got %q", model.ExecutionTimeUnitMilliseconds, result.ExecutionTimeUnit)
	}
	if result.ExecutionTimeScope != model.ExecutionTimeScopeStream {
		t.Fatalf("expected execution_time_scope %q, got %q", model.ExecutionTimeScopeStream, result.ExecutionTimeScope)
	}
}
