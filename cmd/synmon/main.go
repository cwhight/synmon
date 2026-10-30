package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/cwhight/synmon/internal/config"
	"github.com/cwhight/synmon/internal/monitor"
	"github.com/cwhight/synmon/internal/tracetest"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic("load config: " + err.Error())
	}

	logger := buildLogger(cfg.LogLevel)
	defer logger.Sync()

	client := tracetest.NewClient(cfg.TracetestEndpoint)

	scenarios, err := loadScenarios(cfg)
	if err != nil {
		logger.Fatal("load scenarios", zap.Error(err))
	}

	mon := monitor.New(client, logger, scenarios, cfg.TestRunTimeout, cfg.TestPollInterval)

	if err := mon.SyncTests(); err != nil {
		logger.Fatal("sync test definitions", zap.Error(err))
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{Addr: cfg.MetricsAddr, Handler: mux}
	go func() {
		logger.Info("metrics server starting", zap.String("addr", cfg.MetricsAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("metrics server", zap.Error(err))
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger.Info("starting synthetic monitor",
		zap.Int("scenarios", len(mon.Scenarios())),
		zap.Duration("interval", cfg.TestInterval),
	)

	runLoop(ctx, mon, cfg.TestInterval, logger)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	server.Shutdown(shutdownCtx)
	logger.Info("shutdown complete")
}

func runLoop(ctx context.Context, mon *monitor.Monitor, interval time.Duration, logger *zap.Logger) {
	mon.RunAll()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("received shutdown signal")
			return
		case <-ticker.C:
			mon.RunAll()
		}
	}
}

func loadScenarios(cfg *config.Config) ([]monitor.TestScenario, error) {
	if cfg.TestsFile != "" {
		return monitor.LoadTestsFromFile(cfg.TestsFile, cfg.AuthHeaders)
	}
	return monitor.ExampleScenarios(cfg.TracetestEndpoint), nil
}

func buildLogger(level string) *zap.Logger {
	lvl := zapcore.InfoLevel
	_ = lvl.UnmarshalText([]byte(level))

	zapCfg := zap.NewProductionConfig()
	zapCfg.Level = zap.NewAtomicLevelAt(lvl)
	zapCfg.EncoderConfig.TimeKey = "ts"
	zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := zapCfg.Build()
	if err != nil {
		panic("build logger: " + err.Error())
	}
	return logger
}
