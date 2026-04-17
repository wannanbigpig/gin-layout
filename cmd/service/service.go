package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/cmd/bootstrapx"
	"github.com/wannanbigpig/gin-layout/data"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	_ "github.com/wannanbigpig/gin-layout/internal/queue/asynqx"
	"github.com/wannanbigpig/gin-layout/internal/routers"
)

const (
	defaultHost            = "0.0.0.0"
	defaultPort            = 9001
	gracefulShutdownTimout = 10 * time.Second
)

var (
	Cmd = bootstrapx.WrapCommand(&cobra.Command{
		Use:     "service",
		Short:   "Start API service",
		Example: "go-layout service -c config.yml",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run()
		},
	}, bootstrapx.Requirements{Data: true, Validator: true, Queue: true, AllowDegradedStartup: true})
	host string
	port int
)

func init() {
	registerFlags()
}

// registerFlags 注册命令行标志
func registerFlags() {
	Cmd.Flags().StringVarP(&host, "host", "H", defaultHost, "监听服务器地址")
	Cmd.Flags().IntVarP(&port, "port", "P", defaultPort, "监听服务器端口")
}

// run 运行服务器
func run() error {
	engine, err := routers.SetRouters()
	if err != nil {
		return fmt.Errorf("build router failed: %w", err)
	}
	address := fmt.Sprintf("%s:%d", host, port)
	server := &http.Server{
		Addr:    address,
		Handler: engine,
	}

	errChan := make(chan error, 1)
	go func() {
		log.Logger.Info("API service starting", zap.String("address", address))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
		close(errChan)
	}()

	return waitForShutdown(server, errChan)
}

func waitForShutdown(server *http.Server, errChan <-chan error) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	select {
	case err, ok := <-errChan:
		if ok && err != nil {
			return err
		}
		return nil
	case sig := <-sigChan:
		log.Logger.Warn("Received API shutdown signal", zap.String("signal", sig.String()))
	}

	ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown http server failed: %w", err)
	}
	if err := data.Shutdown(); err != nil {
		return fmt.Errorf("shutdown data resources failed: %w", err)
	}

	log.Logger.Info("API service stopped gracefully")
	return nil
}
