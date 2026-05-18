package taskcron

import (
	"context"
	"testing"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/config/autoload"
	"github.com/wannanbigpig/gin-layout/internal/queue"
)

func TestRegisterQueueFallbackHandlersRegistersDisabledNonHighRiskCron(t *testing.T) {
	registry := queue.NewRegistry()

	count := RegisterQueueFallbackHandlers(registry, &config.Conf{})
	if count != 1 {
		t.Fatalf("unexpected fallback handler count: got=%d want=1", count)
	}

	entry, ok := findRegistration(registry, TaskCodeCronDemo)
	if !ok {
		t.Fatalf("expected %s fallback handler to be registered", TaskCodeCronDemo)
	}
	if err := entry.Handler(context.Background(), []byte(`{}`)); err != nil {
		t.Fatalf("fallback handler returned error: %v", err)
	}
}

func TestRegisterQueueFallbackHandlersSkipsHighRiskCron(t *testing.T) {
	registry := queue.NewRegistry()

	RegisterQueueFallbackHandlers(registry, &config.Conf{
		AppConfig: autoload.AppConfig{EnableResetSystemCron: true},
	})
	if _, ok := findRegistration(registry, TaskCodeCronResetSystemData); ok {
		t.Fatalf("did not expect %s fallback handler to be registered", TaskCodeCronResetSystemData)
	}
}

func findRegistration(registry queue.Registry, taskType string) (queue.Registration, bool) {
	for _, entry := range registry.Entries() {
		if entry.TaskType == taskType {
			return entry, true
		}
	}
	return queue.Registration{}, false
}
