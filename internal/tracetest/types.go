package tracetest

type TestDefinition struct {
	Type string             `json:"type"`
	Spec TestDefinitionSpec `json:"spec"`
}

type TestDefinitionSpec struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Trigger Trigger    `json:"trigger"`
	Specs   []TestSpec `json:"specs,omitempty"`
}

type Trigger struct {
	Type        string      `json:"type"`
	HTTPRequest HTTPRequest `json:"httpRequest"`
}

type HTTPRequest struct {
	Method  string       `json:"method"`
	URL     string       `json:"url"`
	Body    string       `json:"body,omitempty"`
	Headers []HTTPHeader `json:"headers,omitempty"`
}

type HTTPHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type TestSpec struct {
	Selector   string   `json:"selector"`
	Assertions []string `json:"assertions"`
}

type RunResult struct {
	ID     int    `json:"id"`
	State  string `json:"state"`
	Result struct {
		AllPassed bool `json:"allPassed"`
		Results   []struct {
			Selector struct {
				Query string `json:"query"`
			} `json:"selector"`
			Results []struct {
				Passed bool   `json:"passed"`
				Error  string `json:"error"`
			} `json:"results"`
		} `json:"results"`
	} `json:"result"`
	TriggerResult struct {
		TriggerResult struct {
			StatusCode int `json:"statusCode"`
		} `json:"triggerResult"`
	} `json:"triggerResult"`
}
