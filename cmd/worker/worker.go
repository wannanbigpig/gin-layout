package worker

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/cmd/bootstrapx"
	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	taskcron "github.com/wannanbigpig/gin-layout/internal/cron"
	"github.com/wannanbigpig/gin-layout/internal/jobs"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/queue/asynqx"
)

var Cmd = bootstrapx.WrapCommand(&cobra.Command{
	Use:     "worker",
	Short:   "Start async worker",
	Example: "go-layout worker -c config.yml",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run()
	},
}, bootstrapx.Requirements{Data: true})

func run() error {
	cfg := config.GetConfig()
	if cfg == nil {
		return fmt.Errorf("queue config is not initialized")
	}
	if !cfg.Queue.Enable {
		return fmt.Errorf("queue.enable is false")
	}
	if cfg.Queue.UseDefaultRedis {
		if !cfg.Redis.Enable {
			return fmt.Errorf("queue uses default redis, but redis.enable is false")
		}
		if err := data.GetRedisInitError(); err != nil {
			return fmt.Errorf("redis initialization failed: %w", err)
		}
		if data.RedisClient() == nil {
			return fmt.Errorf("redis client is unavailable")
		}
	}

	registry := jobs.NewRegistry()
	cronFallbackHandlers := 0
	if cfg.Queue.ConsumeCronFallback {
		cronFallbackHandlers = taskcron.RegisterQueueFallbackHandlers(registry, cfg)
	}

	server, mux, err := asynqx.NewServer(cfg, registry)
	if err != nil {
		return err
	}

	log.Logger.Info("Async worker starting",
		zap.Int("concurrency", cfg.Queue.Concurrency),
		zap.Bool("strict_priority", cfg.Queue.StrictPriority),
		zap.Bool("consume_cron_fallback", cfg.Queue.ConsumeCronFallback),
		zap.Int("cron_fallback_handlers", cronFallbackHandlers),
		zap.Any("queues", cfg.Queue.Queues))

	if err := server.Run(mux); err != nil {
		return err
	}
	if err := data.Shutdown(); err != nil {
		return fmt.Errorf("shutdown data resources failed: %w", err)
	}

	log.Logger.Info("Async worker stopped gracefully")
	return nil
}
