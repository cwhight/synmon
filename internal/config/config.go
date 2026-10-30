package config

import (
	"fmt"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v3"
)

type Config struct {
	TracetestEndpoint string            `envconfig:"TRACETEST_API_ENDPOINT" yaml:"tracetest_api_endpoint" required:"true"`
	TestInterval      time.Duration     `envconfig:"TEST_INTERVAL" yaml:"test_interval" default:"5m"`
	MetricsAddr       string            `envconfig:"METRICS_ADDR" yaml:"metrics_addr" default:":2112"`
	TestRunTimeout    time.Duration     `envconfig:"TEST_RUN_TIMEOUT" yaml:"test_run_timeout" default:"60s"`
	TestPollInterval  time.Duration     `envconfig:"TEST_POLL_INTERVAL" yaml:"test_poll_interval" default:"2s"`
	TestsFile         string            `envconfig:"TESTS_FILE" yaml:"tests_file" default:""`
	AuthHeaders       map[string]string `yaml:"auth_headers"`
	LogLevel          string            `envconfig:"LOG_LEVEL" yaml:"log_level" default:"info"`
}

func Load() (*Config, error) {
	var cfg Config

	configFile := os.Getenv("CONFIG_FILE")
	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("read config file %s: %w", configFile, err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse config file %s: %w", configFile, err)
		}
	}

	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("process env config: %w", err)
	}

	return &cfg, nil
}
