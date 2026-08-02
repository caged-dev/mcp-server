package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// ============================================================================
// A2A Types
// ============================================================================

// A2AAgentRegistration represents a registered A2A agent.
type A2AAgentRegistration struct {
	ID             string     `json:"id"`
	AccountID      string     `json:"account_id"`
	Name           string     `json:"name"`
	Description    string     `json:"description,omitempty"`
	PipelineID     *string    `json:"pipeline_id,omitempty"`
	Template       string     `json:"template,omitempty"`
	Skills         []A2ASkill `json:"skills,omitempty"`
	Public         bool       `json:"public"`
	AllowedOrgs    []string   `json:"allowed_orgs,omitempty"`
	MaxCostPerTask float64    `json:"max_cost_per_task,omitempty"`
	RateLimitRPM   int        `json:"rate_limit_rpm,omitempty"`
	Enabled        bool       `json:"enabled"`
	CreatedAt      string     `json:"created_at"`
	UpdatedAt      string     `json:"updated_at"`
}

// A2ASkill describes a skill/capability an agent provides.
type A2ASkill struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// A2AAgentCard is the discovery document for an A2A agent.
type A2AAgentCard struct {
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	URL            string         `json:"url"`
	Version        string         `json:"version"`
	Capabilities   []string       `json:"capabilities,omitempty"`
	InputModes     []string       `json:"input_modes,omitempty"`
	OutputModes    []string       `json:"output_modes,omitempty"`
	StreamingMode  string         `json:"streaming_mode,omitempty"`
	Authentication *A2AAuthConfig `json:"authentication,omitempty"`
	Skills         []A2ASkill     `json:"skills,omitempty"`
	Provider       *A2AProvider   `json:"provider,omitempty"`
}

// A2AAuthConfig describes auth requirements for an agent.
type A2AAuthConfig struct {
	Type    string   `json:"type"`
	Schemes []string `json:"schemes,omitempty"`
}

// A2AProvider describes the agent provider.
type A2AProvider struct {
	Name         string `json:"name"`
	Organization string `json:"organization,omitempty"`
	URL          string `json:"url,omitempty"`
}

// A2ATask represents a task delegated to an A2A agent.
type A2ATask struct {
	ID            string          `json:"id"`
	SessionID     string          `json:"session_id,omitempty"`
	ToAgentURL    string          `json:"to_agent_url"`
	SkillID       string          `json:"skill_id,omitempty"`
	Status        string          `json:"status"`
	StatusMessage string          `json:"status_message,omitempty"`
	Progress      *A2AProgress    `json:"progress,omitempty"`
	Input         json.RawMessage `json:"input,omitempty"`
	Output        json.RawMessage `json:"output,omitempty"`
	Priority      int             `json:"priority,omitempty"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
	StartedAt     *string         `json:"started_at,omitempty"`
	CompletedAt   *string         `json:"completed_at,omitempty"`
}

// A2AProgress tracks progress on a running task.
type A2AProgress struct {
	Percentage  int    `json:"percentage,omitempty"`
	CurrentStep string `json:"current_step,omitempty"`
	Message     string `json:"message,omitempty"`
}

// A2AMessage is a message in an A2A task conversation.
type A2AMessage struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Role      string    `json:"role"`
	Parts     []A2APart `json:"parts"`
	CreatedAt string    `json:"created_at"`
}

// A2APart is a single piece of content in a message.
type A2APart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
	Name     string          `json:"name,omitempty"`
	MimeType string          `json:"mime_type,omitempty"`
}

// ============================================================================
// A2A Request Types
// ============================================================================

// CreateA2AAgentRequest is the request body for creating an A2A agent.
type CreateA2AAgentRequest struct {
	Name           string     `json:"name"`
	Description    string     `json:"description,omitempty"`
	PipelineID     string     `json:"pipeline_id,omitempty"`
	Template       string     `json:"template,omitempty"`
	Skills         []A2ASkill `json:"skills,omitempty"`
	Public         bool       `json:"public,omitempty"`
	AllowedOrgs    []string   `json:"allowed_orgs,omitempty"`
	MaxCostPerTask float64    `json:"max_cost_per_task,omitempty"`
	RateLimitRPM   int        `json:"rate_limit_rpm,omitempty"`
}

// UpdateA2AAgentRequest is the request body for updating an A2A agent.
type UpdateA2AAgentRequest struct {
	Name           *string    `json:"name,omitempty"`
	Description    *string    `json:"description,omitempty"`
	Template       *string    `json:"template,omitempty"`
	Skills         []A2ASkill `json:"skills,omitempty"`
	Public         *bool      `json:"public,omitempty"`
	Enabled        *bool      `json:"enabled,omitempty"`
	MaxCostPerTask *float64   `json:"max_cost_per_task,omitempty"`
	RateLimitRPM   *int       `json:"rate_limit_rpm,omitempty"`
}

// CreateA2ATaskRequest is the request body for creating an A2A task.
type CreateA2ATaskRequest struct {
	SessionID string          `json:"session_id,omitempty"`
	SkillID   string          `json:"skill_id,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Priority  int             `json:"priority,omitempty"`
	Streaming bool            `json:"streaming,omitempty"`
}

// SendA2AMessageRequest is the request body for sending a message.
type SendA2AMessageRequest struct {
	Parts []A2APart `json:"parts"`
}

// ============================================================================
// A2A Agent Registration API Methods
// ============================================================================

// ListA2AAgents lists all A2A agent registrations for the account.
func (c *Client) ListA2AAgents(ctx context.Context) ([]A2AAgentRegistration, error) {
	var agents []A2AAgentRegistration
	if err := c.do(ctx, http.MethodGet, "/v1/a2a/agents", nil, &agents); err != nil {
		return nil, err
	}
	return agents, nil
}

// GetA2AAgent gets an A2A agent registration by ID.
func (c *Client) GetA2AAgent(ctx context.Context, id string) (*A2AAgentRegistration, error) {
	var agent A2AAgentRegistration
	if err := c.do(ctx, http.MethodGet, "/v1/a2a/agents/"+id, nil, &agent); err != nil {
		return nil, err
	}
	return &agent, nil
}

// CreateA2AAgent creates a new A2A agent registration.
func (c *Client) CreateA2AAgent(ctx context.Context, req *CreateA2AAgentRequest) (*A2AAgentRegistration, error) {
	var agent A2AAgentRegistration
	if err := c.do(ctx, http.MethodPost, "/v1/a2a/agents", req, &agent); err != nil {
		return nil, err
	}
	return &agent, nil
}

// UpdateA2AAgent updates an A2A agent registration.
func (c *Client) UpdateA2AAgent(ctx context.Context, id string, req *UpdateA2AAgentRequest) (*A2AAgentRegistration, error) {
	var agent A2AAgentRegistration
	if err := c.do(ctx, http.MethodPut, "/v1/a2a/agents/"+id, req, &agent); err != nil {
		return nil, err
	}
	return &agent, nil
}

// DeleteA2AAgent deletes an A2A agent registration.
func (c *Client) DeleteA2AAgent(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v1/a2a/agents/"+id, nil, nil)
}

// ============================================================================
// A2A Discovery API Methods
// ============================================================================

// DiscoverA2AAgent fetches the Agent Card from a remote A2A agent.
func (c *Client) DiscoverA2AAgent(ctx context.Context, agentURL string) (*A2AAgentCard, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}

	url := agentURL + "/.well-known/agent.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "caged-mcp/1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &DiscoveryError{
			StatusCode: resp.StatusCode,
			URL:        url,
		}
	}

	var card A2AAgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, err
	}

	return &card, nil
}

// DiscoveryError is returned when agent discovery fails.
type DiscoveryError struct {
	StatusCode int
	URL        string
}

func (e *DiscoveryError) Error() string {
	return "discovery failed: " + e.URL + " returned status " + http.StatusText(e.StatusCode)
}

// ============================================================================
// A2A Task API Methods
// ============================================================================

// CreateA2ATask creates a new task on a remote A2A agent.
func (c *Client) CreateA2ATask(ctx context.Context, agentURL string, req *CreateA2ATaskRequest) (*A2ATask, error) {
	httpClient := &http.Client{Timeout: 30 * time.Second}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := agentURL + "/tasks"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "caged-mcp/1.0")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, &TaskError{StatusCode: resp.StatusCode, Operation: "create"}
	}

	var task A2ATask
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, err
	}

	return &task, nil
}

// GetA2ATask gets a task from a remote A2A agent.
func (c *Client) GetA2ATask(ctx context.Context, agentURL, taskID string) (*A2ATask, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}

	url := agentURL + "/tasks/" + taskID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &TaskError{StatusCode: resp.StatusCode, Operation: "get"}
	}

	var task A2ATask
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, err
	}

	return &task, nil
}

// SendA2AMessage sends a message to a task on a remote A2A agent.
func (c *Client) SendA2AMessage(ctx context.Context, agentURL, taskID string, req *SendA2AMessageRequest) (*A2AMessage, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := agentURL + "/tasks/" + taskID + "/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, &TaskError{StatusCode: resp.StatusCode, Operation: "send_message"}
	}

	var msg A2AMessage
	if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// CancelA2ATask cancels a task on a remote A2A agent.
func (c *Client) CancelA2ATask(ctx context.Context, agentURL, taskID, reason string) error {
	httpClient := &http.Client{Timeout: 10 * time.Second}

	body, err := json.Marshal(map[string]string{"reason": reason})
	if err != nil {
		return err
	}

	url := agentURL + "/tasks/" + taskID + "/cancel"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return &TaskError{StatusCode: resp.StatusCode, Operation: "cancel"}
	}

	return nil
}

// TaskError is returned when a task operation fails.
type TaskError struct {
	StatusCode int
	Operation  string
}

func (e *TaskError) Error() string {
	return "task " + e.Operation + " failed with status " + http.StatusText(e.StatusCode)
}
