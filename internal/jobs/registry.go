package jobs

import "github.com/wannanbigpig/gin-layout/internal/queue"

// NewRegistry 创建并注册当前版本的全部任务处理器。
func NewRegistry() queue.Registry {
	registry := queue.NewRegistry()
	RegisterAll(registry)
	return registry
}
