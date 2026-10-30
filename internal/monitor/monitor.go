package monitor

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"

	"github.com/cwhight/synmon/internal/tracetest"
)

var (
	testRunTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "synthetic_monitor_test_runs_total",
		Help: "Total synthetic test runs by test and result",
	}, []string{"test", "result"})

	testRunDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "synthetic_monitor_test_duration_seconds",
		Help:    "Duration of synthetic test runs",
		Buckets: prometheus.DefBuckets,
	}, []string{"test"})
)

type tracetestClient interface {
	UpsertTest(def tracetest.TestDefinition) error
	RunTest(testID string) (int, error)
	WaitForRun(testID string, runID int, timeout, pollInterval time.Duration) (*tracetest.RunResult, error)
}

type TestScenario struct {
	ID         string                   `yaml:"id"`
	Name       string                   `yaml:"name"`
	Definition tracetest.TestDefinition `yaml:"definition"`
}

type Monitor struct {
	client       tracetestClient
	logger       *zap.Logger
	scenarios    []TestScenario
	runTimeout   time.Duration
	pollInterval time.Duration
}

func New(client tracetestClient, logger *zap.Logger, scenarios []TestScenario, runTimeout, pollInterval time.Duration) *Monitor {
	return &Monitor{
		client:       client,
		logger:       logger,
		scenarios:    scenarios,
		runTimeout:   runTimeout,
		pollInterval: pollInterval,
	}
}

func (m *Monitor) SyncTests() error {
	for _, s := range m.scenarios {
		if err := m.client.UpsertTest(s.Definition); err != nil {
			return fmt.Errorf("upsert %s: %w", s.ID, err)
		}
		m.logger.Debug("synced test definition", zap.String("test", s.ID))
	}
	return nil
}

func (m *Monitor) RunAll() {
	for _, s := range m.scenarios {
		m.runSingle(s)
	}
}

func (m *Monitor) Scenarios() []TestScenario {
	return m.scenarios
}

func (m *Monitor) runSingle(scenario TestScenario) {
	start := time.Now()

	runID, err := m.client.RunTest(scenario.ID)
	if err != nil {
		testRunTotal.WithLabelValues(scenario.ID, "error").Inc()
		m.logger.Error("trigger test", zap.String("test", scenario.ID), zap.Error(err))
		return
	}

	result, err := m.client.WaitForRun(scenario.ID, runID, m.runTimeout, m.pollInterval)
	duration := time.Since(start)
	testRunDuration.WithLabelValues(scenario.ID).Observe(duration.Seconds())

	if err != nil {
		testRunTotal.WithLabelValues(scenario.ID, "error").Inc()
		m.logger.Error("wait for test run", zap.String("test", scenario.ID), zap.Error(err))
		return
	}

	if result.Result.AllPassed {
		testRunTotal.WithLabelValues(scenario.ID, "pass").Inc()
		m.logger.Info("test passed",
			zap.String("test", scenario.ID),
			zap.Duration("duration", duration),
		)
	} else {
		testRunTotal.WithLabelValues(scenario.ID, "fail").Inc()
		m.logger.Warn("test failed",
			zap.String("test", scenario.ID),
			zap.Duration("duration", duration),
			zap.Int("run_id", runID),
		)
	}
}
