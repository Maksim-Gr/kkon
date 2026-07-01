package connector

// ConnectorState represents the state of a connector instance.
//
//nolint:revive
type ConnectorState struct {
	State    string `json:"state"`
	WorkerID string `json:"worker_id"`
}

// TaskState represents the state of a single connector task.
type TaskState struct {
	ID       int    `json:"id"`
	State    string `json:"state"`
	WorkerID string `json:"worker_id"`
}

// Status is the response from GET /connectors/{name}/status.
type Status struct {
	Name      string         `json:"name"`
	Connector ConnectorState `json:"connector"`
	Tasks     []TaskState    `json:"tasks"`
	Type      string         `json:"type"`
}

// ConnectorInfo is the response from POST /connectors and GET /connectors/{name}.
//
//nolint:revive
type ConnectorInfo struct {
	Name   string            `json:"name"`
	Config map[string]string `json:"config"`
	Tasks  []TaskRef         `json:"tasks"`
	Type   string            `json:"type"`
}

// ErrorResponse is the structured error body returned by Kafka Connect on 4xx/5xx responses.
type ErrorResponse struct {
	ErrorCode int    `json:"error_code"`
	Message   string `json:"message"`
}

// ConnectorExpanded is the per-connector shape returned by GET /connectors?expand=status&expand=info.
type ConnectorExpanded struct {
	Info   ConnectorInfo `json:"info"`
	Status Status        `json:"status"`
}

// ConnectorsStatusResponse maps connector name to its Status.
type ConnectorsStatusResponse map[string]Status
