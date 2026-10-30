package monitor

import (
	"fmt"
	"os"

	"github.com/cwhight/synmon/internal/tracetest"
	"gopkg.in/yaml.v3"
)

type TestsFile struct {
	Tests []TestFileEntry `yaml:"tests"`
}

type TestFileEntry struct {
	ID      string            `yaml:"id"`
	Name    string            `yaml:"name"`
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Body    string            `yaml:"body,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Specs   []TestFileSpec    `yaml:"specs,omitempty"`
}

type TestFileSpec struct {
	Selector   string   `yaml:"selector"`
	Assertions []string `yaml:"assertions"`
}

func LoadTestsFromFile(path string, globalHeaders map[string]string) ([]TestScenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read tests file %s: %w", path, err)
	}

	expanded := os.ExpandEnv(string(data))

	var tf TestsFile
	if err := yaml.Unmarshal([]byte(expanded), &tf); err != nil {
		return nil, fmt.Errorf("parse tests file %s: %w", path, err)
	}

	scenarios := make([]TestScenario, 0, len(tf.Tests))
	for _, t := range tf.Tests {
		headers := make([]tracetest.HTTPHeader, 0, len(globalHeaders)+len(t.Headers))

		for k, v := range globalHeaders {
			headers = append(headers, tracetest.HTTPHeader{Key: k, Value: v})
		}
		for k, v := range t.Headers {
			headers = append(headers, tracetest.HTTPHeader{Key: k, Value: v})
		}

		specs := make([]tracetest.TestSpec, 0, len(t.Specs))
		for _, s := range t.Specs {
			specs = append(specs, tracetest.TestSpec{
				Selector:   s.Selector,
				Assertions: s.Assertions,
			})
		}

		scenarios = append(scenarios, TestScenario{
			ID:   t.ID,
			Name: t.Name,
			Definition: tracetest.TestDefinition{
				Type: "Test",
				Spec: tracetest.TestDefinitionSpec{
					ID:   t.ID,
					Name: t.Name,
					Trigger: tracetest.Trigger{
						Type: "http",
						HTTPRequest: tracetest.HTTPRequest{
							Method:  t.Method,
							URL:     t.URL,
							Body:    t.Body,
							Headers: headers,
						},
					},
					Specs: specs,
				},
			},
		})
	}

	return scenarios, nil
}

func ExampleScenarios(baseURL string) []TestScenario {
	return []TestScenario{
		{
			ID:   "synthetic-health-check",
			Name: "Health Check",
			Definition: tracetest.TestDefinition{
				Type: "Test",
				Spec: tracetest.TestDefinitionSpec{
					ID:   "synthetic-health-check",
					Name: "Synthetic: Health Check",
					Trigger: tracetest.Trigger{
						Type: "http",
						HTTPRequest: tracetest.HTTPRequest{
							Method: "GET",
							URL:    baseURL + "/health",
							Headers: []tracetest.HTTPHeader{
								{Key: "Content-Type", Value: "application/json"},
							},
						},
					},
					Specs: []tracetest.TestSpec{
						{
							Selector:   `span[name = "GET /health"]`,
							Assertions: []string{"attr:tracetest.selected_spans.count > 0"},
						},
					},
				},
			},
		},
	}
}
