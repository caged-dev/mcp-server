package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/caged-dev/mcp-server/internal/api"
	"github.com/caged-dev/mcp-server/internal/mcp"
)

// PipelineListTool lists all pipelines.
type PipelineListTool struct {
	client *api.Client
}

func (t *PipelineListTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "pipeline_list",
		Description: "List all pipelines for your account.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}
}

func (t *PipelineListTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	pipelines, err := t.client.ListPipelines(ctx)
	if err != nil {
		return errorResult("listing pipelines: " + err.Error())
	}

	if len(pipelines) == 0 {
		return textResult("No pipelines found")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d pipeline(s):\n\n", len(pipelines)))
	for _, p := range pipelines {
		sb.WriteString(fmt.Sprintf("- **%s** (ID: %s)\n", p.Name, p.ID))
		if p.Description != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", p.Description))
		}
		sb.WriteString(fmt.Sprintf("  Stages: %d | Created: %s\n", len(p.Stages), p.CreatedAt))
	}
	return textResult(sb.String())
}

// PipelineGetTool gets pipeline details.
type PipelineGetTool struct {
	client *api.Client
}

func (t *PipelineGetTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "pipeline_get",
		Description: "Get details of a specific pipeline.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pipeline_id": map[string]string{"type": "string", "description": "Pipeline ID (UUID)"},
			},
			"required": []string{"pipeline_id"},
		},
	}
}

func (t *PipelineGetTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		PipelineID string `json:"pipeline_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.PipelineID == "" {
		return errorResult("pipeline_id is required")
	}

	pipeline, err := t.client.GetPipeline(ctx, params.PipelineID)
	if err != nil {
		return errorResult("getting pipeline: " + err.Error())
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", pipeline.Name))
	sb.WriteString(fmt.Sprintf("**ID:** %s\n", pipeline.ID))
	if pipeline.Description != "" {
		sb.WriteString(fmt.Sprintf("**Description:** %s\n", pipeline.Description))
	}
	sb.WriteString(fmt.Sprintf("**Created:** %s\n\n", pipeline.CreatedAt))

	sb.WriteString("## Stages\n\n")
	for i, s := range pipeline.Stages {
		sb.WriteString(fmt.Sprintf("%d. **%s** [%s]\n", i+1, s.Name, s.Type))
		if s.Command != "" {
			sb.WriteString(fmt.Sprintf("   Command: `%s`\n", s.Command))
		}
		if len(s.DependsOn) > 0 {
			sb.WriteString(fmt.Sprintf("   Depends on: %s\n", strings.Join(s.DependsOn, ", ")))
		}
	}
	return textResult(sb.String())
}

// PipelineCreateTool creates a new pipeline.
type PipelineCreateTool struct {
	client *api.Client
}

func (t *PipelineCreateTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "pipeline_create",
		Description: "Create a new pipeline with stages.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":        map[string]string{"type": "string", "description": "Pipeline name"},
				"description": map[string]string{"type": "string", "description": "Pipeline description"},
				"stages": map[string]interface{}{
					"type":        "array",
					"description": "Pipeline stages",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name":         map[string]string{"type": "string", "description": "Stage name"},
							"type":         map[string]string{"type": "string", "description": "Stage type (command, approval, gate, eval)"},
							"command":      map[string]string{"type": "string", "description": "Command to execute (for command stages)"},
							"template":     map[string]string{"type": "string", "description": "Sandbox template (node, python, etc.)"},
							"timeout":      map[string]string{"type": "string", "description": "Stage timeout (e.g., 5m, 1h)"},
							"depends_on":   map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}, "description": "Stage dependencies"},
							"max_attempts": map[string]string{"type": "integer", "description": "Maximum retry attempts"},
						},
						"required": []string{"name", "type"},
					},
				},
			},
			"required": []string{"name", "stages"},
		},
	}
}

func (t *PipelineCreateTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		Name        string                `json:"name"`
		Description string                `json:"description"`
		Stages      []api.StageDefinition `json:"stages"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.Name == "" {
		return errorResult("name is required")
	}
	if len(params.Stages) == 0 {
		return errorResult("at least one stage is required")
	}

	pipeline, err := t.client.CreatePipeline(ctx, &api.CreatePipelineRequest{
		Name:        params.Name,
		Description: params.Description,
		Stages:      params.Stages,
	})
	if err != nil {
		return errorResult("creating pipeline: " + err.Error())
	}

	return textResult(fmt.Sprintf("Pipeline created successfully!\n\n**Name:** %s\n**ID:** %s\n**Stages:** %d", pipeline.Name, pipeline.ID, len(pipeline.Stages)))
}

// PipelineDeleteTool deletes a pipeline.
type PipelineDeleteTool struct {
	client *api.Client
}

func (t *PipelineDeleteTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "pipeline_delete",
		Description: "Delete a pipeline.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pipeline_id": map[string]string{"type": "string", "description": "Pipeline ID (UUID)"},
			},
			"required": []string{"pipeline_id"},
		},
	}
}

func (t *PipelineDeleteTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		PipelineID string `json:"pipeline_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.PipelineID == "" {
		return errorResult("pipeline_id is required")
	}

	if err := t.client.DeletePipeline(ctx, params.PipelineID); err != nil {
		return errorResult("deleting pipeline: " + err.Error())
	}

	return textResult(fmt.Sprintf("Pipeline %s deleted successfully", params.PipelineID))
}

// PipelineRunTool starts a pipeline run.
type PipelineRunTool struct {
	client *api.Client
}

func (t *PipelineRunTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "pipeline_run",
		Description: "Start a new pipeline run.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pipeline_id": map[string]string{"type": "string", "description": "Pipeline ID (UUID)"},
				"repo":        map[string]string{"type": "string", "description": "Git repository URL to clone"},
				"branch":      map[string]string{"type": "string", "description": "Git branch to checkout"},
				"env": map[string]interface{}{
					"type":        "object",
					"description": "Environment variables to pass to stages",
					"additionalProperties": map[string]string{"type": "string"},
				},
			},
			"required": []string{"pipeline_id"},
		},
	}
}

func (t *PipelineRunTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		PipelineID string            `json:"pipeline_id"`
		Repo       string            `json:"repo"`
		Branch     string            `json:"branch"`
		Env        map[string]string `json:"env"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.PipelineID == "" {
		return errorResult("pipeline_id is required")
	}

	run, err := t.client.StartRun(ctx, params.PipelineID, &api.StartRunRequest{
		Trigger: "mcp",
		Repo:    params.Repo,
		Branch:  params.Branch,
		Env:     params.Env,
	})
	if err != nil {
		return errorResult("starting run: " + err.Error())
	}

	return textResult(fmt.Sprintf("Pipeline run started!\n\n**Run ID:** %s\n**Status:** %s\n**Trigger:** %s", run.ID, run.Status, run.Trigger))
}

// PipelineRunsListTool lists runs for a pipeline.
type PipelineRunsListTool struct {
	client *api.Client
}

func (t *PipelineRunsListTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "pipeline_runs_list",
		Description: "List runs for a pipeline.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pipeline_id": map[string]string{"type": "string", "description": "Pipeline ID (UUID)"},
			},
			"required": []string{"pipeline_id"},
		},
	}
}

func (t *PipelineRunsListTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		PipelineID string `json:"pipeline_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.PipelineID == "" {
		return errorResult("pipeline_id is required")
	}

	runs, err := t.client.ListRuns(ctx, params.PipelineID)
	if err != nil {
		return errorResult("listing runs: " + err.Error())
	}

	if len(runs) == 0 {
		return textResult("No runs found for this pipeline")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d run(s):\n\n", len(runs)))
	for _, r := range runs {
		icon := statusIcon(r.Status)
		sb.WriteString(fmt.Sprintf("- %s **%s** (%s)\n", icon, r.ID[:8], r.Status))
		sb.WriteString(fmt.Sprintf("  Trigger: %s | Created: %s\n", r.Trigger, r.CreatedAt))
	}
	return textResult(sb.String())
}

func statusIcon(status string) string {
	switch status {
	case "succeeded":
		return "✅"
	case "failed":
		return "❌"
	case "running":
		return "🔄"
	case "paused":
		return "⏸️"
	case "canceled":
		return "🚫"
	default:
		return "⏳"
	}
}

// PipelineRunGetTool gets details of a specific run.
type PipelineRunGetTool struct {
	client *api.Client
}

func (t *PipelineRunGetTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "pipeline_run_get",
		Description: "Get details of a specific pipeline run.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pipeline_id": map[string]string{"type": "string", "description": "Pipeline ID (UUID)"},
				"run_id":      map[string]string{"type": "string", "description": "Run ID (UUID)"},
			},
			"required": []string{"pipeline_id", "run_id"},
		},
	}
}

func (t *PipelineRunGetTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		PipelineID string `json:"pipeline_id"`
		RunID      string `json:"run_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.PipelineID == "" {
		return errorResult("pipeline_id is required")
	}
	if params.RunID == "" {
		return errorResult("run_id is required")
	}

	run, err := t.client.GetRun(ctx, params.PipelineID, params.RunID)
	if err != nil {
		return errorResult("getting run: " + err.Error())
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Run %s\n\n", run.ID[:8]))
	sb.WriteString(fmt.Sprintf("**Status:** %s %s\n", statusIcon(run.Status), run.Status))
	sb.WriteString(fmt.Sprintf("**Pipeline:** %s\n", run.PipelineID))
	sb.WriteString(fmt.Sprintf("**Trigger:** %s\n", run.Trigger))
	sb.WriteString(fmt.Sprintf("**Created:** %s\n", run.CreatedAt))
	if run.StartedAt != "" {
		sb.WriteString(fmt.Sprintf("**Started:** %s\n", run.StartedAt))
	}
	if run.EndedAt != "" {
		sb.WriteString(fmt.Sprintf("**Ended:** %s\n", run.EndedAt))
	}
	return textResult(sb.String())
}

// PipelineRunCancelTool cancels a pipeline run.
type PipelineRunCancelTool struct {
	client *api.Client
}

func (t *PipelineRunCancelTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "pipeline_run_cancel",
		Description: "Cancel a running pipeline run.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pipeline_id": map[string]string{"type": "string", "description": "Pipeline ID (UUID)"},
				"run_id":      map[string]string{"type": "string", "description": "Run ID (UUID)"},
			},
			"required": []string{"pipeline_id", "run_id"},
		},
	}
}

func (t *PipelineRunCancelTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		PipelineID string `json:"pipeline_id"`
		RunID      string `json:"run_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.PipelineID == "" {
		return errorResult("pipeline_id is required")
	}
	if params.RunID == "" {
		return errorResult("run_id is required")
	}

	if err := t.client.CancelRun(ctx, params.PipelineID, params.RunID); err != nil {
		return errorResult("canceling run: " + err.Error())
	}

	return textResult(fmt.Sprintf("Run %s canceled successfully", params.RunID[:8]))
}

// NewPipelineTools creates all pipeline tools using the given API client.
func NewPipelineTools(client *api.Client) []mcp.Tool {
	return []mcp.Tool{
		&PipelineListTool{client: client},
		&PipelineGetTool{client: client},
		&PipelineCreateTool{client: client},
		&PipelineDeleteTool{client: client},
		&PipelineRunTool{client: client},
		&PipelineRunsListTool{client: client},
		&PipelineRunGetTool{client: client},
		&PipelineRunCancelTool{client: client},
	}
}
