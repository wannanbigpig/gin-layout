package bootstrapx

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

func TestWrapCommandReturnsDataErrorWhenDegradedStartupDisabled(t *testing.T) {
	restoreInit := stubBootstrapInitializers(
		func() error { return errTestDataInit },
		func() error { return nil },
		func() error { return nil },
	)
	defer restoreInit()

	restoreConfig := setAllowDegradedStartup(t, false)
	defer restoreConfig()

	cmd := WrapCommand(&cobra.Command{Use: "service"}, Requirements{
		Data:                 true,
		AllowDegradedStartup: true,
	})

	err := cmd.PreRunE(cmd, nil)
	if !errors.Is(err, errTestDataInit) {
		t.Fatalf("expected data init error, got %v", err)
	}
}

func TestWrapCommandAllowsDataAndQueueErrorsWhenEnabled(t *testing.T) {
	dataCalled := false
	validatorCalled := false
	queueCalled := false
	originalCalled := false

	restoreInit := stubBootstrapInitializers(
		func() error {
			dataCalled = true
			return errTestDataInit
		},
		func() error {
			validatorCalled = true
			return nil
		},
		func() error {
			queueCalled = true
			return errTestQueueInit
		},
	)
	defer restoreInit()

	restoreConfig := setAllowDegradedStartup(t, true)
	defer restoreConfig()

	cmd := WrapCommand(&cobra.Command{
		Use: "service",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			originalCalled = true
			return nil
		},
	}, Requirements{
		Data:                 true,
		Validator:            true,
		Queue:                true,
		AllowDegradedStartup: true,
	})

	if err := cmd.PreRunE(cmd, nil); err != nil {
		t.Fatalf("expected degraded startup to continue, got %v", err)
	}
	if !dataCalled || !validatorCalled || !queueCalled {
		t.Fatalf("expected all initializers to run, got data=%v validator=%v queue=%v", dataCalled, validatorCalled, queueCalled)
	}
	if !originalCalled {
		t.Fatal("expected original PreRunE to be called")
	}
}

func TestWrapCommandKeepsStrictModeForCommandsWithoutOptIn(t *testing.T) {
	restoreInit := stubBootstrapInitializers(
		func() error { return errTestDataInit },
		func() error { return nil },
		func() error { return nil },
	)
	defer restoreInit()

	restoreConfig := setAllowDegradedStartup(t, true)
	defer restoreConfig()

	cmd := WrapCommand(&cobra.Command{Use: "cron"}, Requirements{
		Data: true,
	})

	err := cmd.PreRunE(cmd, nil)
	if !errors.Is(err, errTestDataInit) {
		t.Fatalf("expected strict command to return data init error, got %v", err)
	}
}

func TestWrapCommandKeepsValidatorStrictEvenWhenDegradedStartupEnabled(t *testing.T) {
	restoreInit := stubBootstrapInitializers(
		func() error { return errTestDataInit },
		func() error { return errTestValidatorInit },
		func() error { return nil },
	)
	defer restoreInit()

	restoreConfig := setAllowDegradedStartup(t, true)
	defer restoreConfig()

	cmd := WrapCommand(&cobra.Command{Use: "service"}, Requirements{
		Data:                 true,
		Validator:            true,
		AllowDegradedStartup: true,
	})

	err := cmd.PreRunE(cmd, nil)
	if !errors.Is(err, errTestValidatorInit) {
		t.Fatalf("expected validator init error, got %v", err)
	}
}

var (
	errTestDataInit      = errors.New("data init failed")
	errTestQueueInit     = errors.New("queue init failed")
	errTestValidatorInit = errors.New("validator init failed")
)

func stubBootstrapInitializers(dataFn, validatorFn, queueFn func() error) func() {
	previousData := initializeDataFunc
	previousValidator := initializeValidatorFunc
	previousQueue := initializeQueueFunc
	initializeDataFunc = dataFn
	initializeValidatorFunc = validatorFn
	initializeQueueFunc = queueFn

	return func() {
		initializeDataFunc = previousData
		initializeValidatorFunc = previousValidator
		initializeQueueFunc = previousQueue
	}
}

func setAllowDegradedStartup(t *testing.T, enabled bool) func() {
	t.Helper()

	originalConfig := config.Config
	originalLogger := log.Logger
	cloned := *config.Config
	cloned.AllowDegradedStartup = enabled
	config.Config = &cloned
	log.Logger = zap.NewNop()

	return func() {
		config.Config = originalConfig
		log.Logger = originalLogger
	}
}
