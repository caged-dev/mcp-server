// Package api provides an HTTP client for the Caged API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for the Caged API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

const defaultTimeout = 30 * time.Second

// NewClient creates a new API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// Pipeline represents a pipeline definition.
type Pipeline struct {
	ID          string            `json:"id"`
	AccountID   string            `json:"account_id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Stages      []StageDefinition `json:"stages"`
	Defaults    StageDefaults     `json:"defaults,omitempty"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// StageDefinition describes a pipeline stage.
type StageDefinition struct {
	Name        string          `json:"name"`
	Type        string          `json:"type"`
	Command     string          `json:"command,omitempty"`
	Template    string          `json:"template,omitempty"`
	Timeout     string          `json:"timeout,omitempty"`
	DependsOn   []string        `json:"depends_on,omitempty"`
	RequireAck  bool            `json:"require_ack,omitempty"`
	Condition   *StageCondition `json:"condition,omitempty"`
	MaxAttempts int             `json:"max_attempts,omitempty"`
}

// StageCondition configures conditional stage execution.
type StageCondition struct {
	OnSuccess bool   `json:"on_success,omitempty"`
	OnFailure bool   `json:"on_failure,omitempty"`
	If        string `json:"if,omitempty"`
}

// StageDefaults holds default values for stages.
type StageDefaults struct {
	Template    string `json:"template,omitempty"`
	Timeout     string `json:"timeout,omitempty"`
	MaxAttempts int    `json:"max_attempts,omitempty"`
}

// Run represents a pipeline run.
type Run struct {
	ID         string     `json:"id"`
	PipelineID string     `json:"pipeline_id"`
	Status     string     `json:"status"`
	Trigger    string     `json:"trigger"`
	Input      RunInput   `json:"input,omitempty"`
	Output     *RunOutput `json:"output,omitempty"`
	StartedAt  string     `json:"started_at,omitempty"`
	EndedAt    string     `json:"ended_at,omitempty"`
	CreatedAt  string     `json:"created_at"`
}

// RunInput is input to a pipeline run.
type RunInput struct {
	Env    map[string]string `json:"env,omitempty"`
	Repo   string            `json:"repo,omitempty"`
	Branch string            `json:"branch,omitempty"`
}

// RunOutput holds outputs from a completed run.
type RunOutput struct {
	State map[string]any `json:"state,omitempty"`
}

// CreatePipelineRequest is the request body for creating a pipeline.
type CreatePipelineRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Stages      []StageDefinition `json:"stages"`
	Defaults    StageDefaults     `json:"defaults,omitempty"`
}

// StartRunRequest is the request body for starting a pipeline run.
type StartRunRequest struct {
	Trigger string            `json:"trigger,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Repo    string            `json:"repo,omitempty"`
	Branch  string            `json:"branch,omitempty"`
}

// CreatePipeline creates a new pipeline.
func (c *Client) CreatePipeline(ctx context.Context, req *CreatePipelineRequest) (*Pipeline, error) {
	var p Pipeline
	if err := c.do(ctx, http.MethodPost, "/v1/pipelines", req, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// ListPipelines lists all pipelines.
func (c *Client) ListPipelines(ctx context.Context) ([]Pipeline, error) {
	var pipelines []Pipeline
	if err := c.do(ctx, http.MethodGet, "/v1/pipelines", nil, &pipelines); err != nil {
		return nil, err
	}
	return pipelines, nil
}

// GetPipeline gets a pipeline by ID.
func (c *Client) GetPipeline(ctx context.Context, id string) (*Pipeline, error) {
	var p Pipeline
	if err := c.do(ctx, http.MethodGet, "/v1/pipelines/"+id, nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// DeletePipeline deletes a pipeline by ID.
func (c *Client) DeletePipeline(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v1/pipelines/"+id, nil, nil)
}

// StartRun starts a new pipeline run.
func (c *Client) StartRun(ctx context.Context, pipelineID string, req *StartRunRequest) (*Run, error) {
	var r Run
	if err := c.do(ctx, http.MethodPost, "/v1/pipelines/"+pipelineID+"/runs", req, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ListRuns lists runs for a pipeline.
func (c *Client) ListRuns(ctx context.Context, pipelineID string) ([]Run, error) {
	var runs []Run
	if err := c.do(ctx, http.MethodGet, "/v1/pipelines/"+pipelineID+"/runs", nil, &runs); err != nil {
		return nil, err
	}
	return runs, nil
}

// GetRun gets a run by ID.
func (c *Client) GetRun(ctx context.Context, pipelineID, runID string) (*Run, error) {
	var r Run
	if err := c.do(ctx, http.MethodGet, "/v1/pipelines/"+pipelineID+"/runs/"+runID, nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// CancelRun cancels a pipeline run.
func (c *Client) CancelRun(ctx context.Context, pipelineID, runID string) error {
	return c.do(ctx, http.MethodPost, "/v1/pipelines/"+pipelineID+"/runs/"+runID+"/cancel", nil, nil)
}

func (c *Client) do(ctx context.Context, method, path string, body any, result any) error {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "caged-mcp")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}
