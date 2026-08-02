package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/caged-dev/mcp-server/internal/api"
	"github.com/caged-dev/mcp-server/internal/mcp"
)

// ============================================================================
// Agent Registration Tools
// ============================================================================

// A2AAgentsListTool lists all A2A agent registrations.
type A2AAgentsListTool struct {
	client *api.Client
}

func (t *A2AAgentsListTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "a2a_agents_list",
		Description: "List all A2A agent registrations for your account.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	}
}

func (t *A2AAgentsListTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	agents, err := t.client.ListA2AAgents(ctx)
	if err != nil {
		return errorResult("listing A2A agents: " + err.Error())
	}

	if len(agents) == 0 {
		return textResult("No A2A agents registered")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d A2A agent(s):\n\n", len(agents)))
	for _, a := range agents {
		status := "disabled"
		if a.Enabled {
			status = "enabled"
		}
		visibility := "private"
		if a.Public {
			visibility = "public"
		}
		sb.WriteString(fmt.Sprintf("- **%s** (ID: %s)\n", a.Name, a.ID))
		sb.WriteString(fmt.Sprintf("  Status: %s | Visibility: %s\n", status, visibility))
		if a.Description != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", a.Description))
		}
		if len(a.Skills) > 0 {
			sb.WriteString(fmt.Sprintf("  Skills: %d\n", len(a.Skills)))
		}
	}
	return textResult(sb.String())
}

// A2AAgentGetTool gets details of an A2A agent.
type A2AAgentGetTool struct {
	client *api.Client
}

func (t *A2AAgentGetTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "a2a_agent_get",
		Description: "Get details of a specific A2A agent registration.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"agent_id": map[string]string{"type": "string", "description": "Agent registration ID (UUID)"},
			},
			"required": []string{"agent_id"},
		},
	}
}

func (t *A2AAgentGetTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.AgentID == "" {
		return errorResult("agent_id is required")
	}

	agent, err := t.client.GetA2AAgent(ctx, params.AgentID)
	if err != nil {
		return errorResult("getting A2A agent: " + err.Error())
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", agent.Name))
	sb.WriteString(fmt.Sprintf("**ID:** %s\n", agent.ID))
	if agent.Description != "" {
		sb.WriteString(fmt.Sprintf("**Description:** %s\n", agent.Description))
	}
	sb.WriteString(fmt.Sprintf("**Enabled:** %v\n", agent.Enabled))
	sb.WriteString(fmt.Sprintf("**Public:** %v\n", agent.Public))
	if agent.Template != "" {
		sb.WriteString(fmt.Sprintf("**Template:** %s\n", agent.Template))
	}
	if agent.MaxCostPerTask > 0 {
		sb.WriteString(fmt.Sprintf("**Max Cost/Task:** $%.2f\n", agent.MaxCostPerTask))
	}
	if agent.RateLimitRPM > 0 {
		sb.WriteString(fmt.Sprintf("**Rate Limit:** %d RPM\n", agent.RateLimitRPM))
	}

	if len(agent.Skills) > 0 {
		sb.WriteString("\n## Skills\n\n")
		for _, s := range agent.Skills {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", s.Name, s.Description))
			if len(s.Tags) > 0 {
				sb.WriteString(fmt.Sprintf("  Tags: %s\n", strings.Join(s.Tags, ", ")))
			}
		}
	}

	sb.WriteString(fmt.Sprintf("\n**Created:** %s\n", agent.CreatedAt))
	return textResult(sb.String())
}

// A2AAgentCreateTool creates a new A2A agent registration.
type A2AAgentCreateTool struct {
	client *api.Client
}

func (t *A2AAgentCreateTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "a2a_agent_create",
		Description: "Register a new A2A agent that can be discovered and delegated to by other agents.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":              map[string]string{"type": "string", "description": "Agent name"},
				"description":       map[string]string{"type": "string", "description": "Agent description"},
				"pipeline_id":       map[string]string{"type": "string", "description": "Pipeline ID to execute for tasks"},
				"template":          map[string]string{"type": "string", "description": "Sandbox template (node, python, etc.)"},
				"public":            map[string]string{"type": "boolean", "description": "Make agent publicly discoverable"},
				"max_cost_per_task": map[string]string{"type": "number", "description": "Maximum cost per task in USD"},
				"rate_limit_rpm":    map[string]string{"type": "integer", "description": "Rate limit (requests per minute)"},
				"skills": map[string]interface{}{
					"type":        "array",
					"description": "Skills this agent provides",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id":          map[string]string{"type": "string", "description": "Skill ID"},
							"name":        map[string]string{"type": "string", "description": "Skill name"},
							"description": map[string]string{"type": "string", "description": "Skill description"},
							"tags":        map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}},
						},
						"required": []string{"id", "name"},
					},
				},
			},
			"required": []string{"name"},
		},
	}
}

func (t *A2AAgentCreateTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params api.CreateA2AAgentRequest
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.Name == "" {
		return errorResult("name is required")
	}

	agent, err := t.client.CreateA2AAgent(ctx, &params)
	if err != nil {
		return errorResult("creating A2A agent: " + err.Error())
	}

	return textResult(fmt.Sprintf("A2A agent registered!\n\n**ID:** %s\n**Name:** %s\n**Public:** %v", agent.ID, agent.Name, agent.Public))
}

// A2AAgentDeleteTool deletes an A2A agent registration.
type A2AAgentDeleteTool struct {
	client *api.Client
}

func (t *A2AAgentDeleteTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "a2a_agent_delete",
		Description: "Delete an A2A agent registration.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"agent_id": map[string]string{"type": "string", "description": "Agent registration ID (UUID)"},
			},
			"required": []string{"agent_id"},
		},
	}
}

func (t *A2AAgentDeleteTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.AgentID == "" {
		return errorResult("agent_id is required")
	}

	if err := t.client.DeleteA2AAgent(ctx, params.AgentID); err != nil {
		return errorResult("deleting A2A agent: " + err.Error())
	}

	return textResult(fmt.Sprintf("A2A agent %s deleted", params.AgentID))
}

// ============================================================================
// Agent Discovery Tools
// ============================================================================

// A2ADiscoverTool discovers a remote A2A agent.
type A2ADiscoverTool struct {
	client *api.Client
}

func (t *A2ADiscoverTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "a2a_discover",
		Description: "Discover a remote A2A agent by fetching its Agent Card.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]string{"type": "string", "description": "Agent URL (e.g., https://agent.example.com)"},
			},
			"required": []string{"url"},
		},
	}
}

func (t *A2ADiscoverTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.URL == "" {
		return errorResult("url is required")
	}

	card, err := t.client.DiscoverA2AAgent(ctx, params.URL)
	if err != nil {
		return errorResult("discovering agent: " + err.Error())
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", card.Name))
	sb.WriteString(fmt.Sprintf("**URL:** %s\n", card.URL))
	sb.WriteString(fmt.Sprintf("**Version:** %s\n", card.Version))
	if card.Description != "" {
		sb.WriteString(fmt.Sprintf("**Description:** %s\n", card.Description))
	}

	if len(card.Capabilities) > 0 {
		sb.WriteString(fmt.Sprintf("**Capabilities:** %s\n", strings.Join(card.Capabilities, ", ")))
	}
	if len(card.InputModes) > 0 {
		sb.WriteString(fmt.Sprintf("**Input Modes:** %s\n", strings.Join(card.InputModes, ", ")))
	}
	if len(card.OutputModes) > 0 {
		sb.WriteString(fmt.Sprintf("**Output Modes:** %s\n", strings.Join(card.OutputModes, ", ")))
	}

	if card.Authentication != nil {
		sb.WriteString(fmt.Sprintf("**Auth:** %s", card.Authentication.Type))
		if len(card.Authentication.Schemes) > 0 {
			sb.WriteString(fmt.Sprintf(" (%s)", strings.Join(card.Authentication.Schemes, ", ")))
		}
		sb.WriteString("\n")
	}

	if len(card.Skills) > 0 {
		sb.WriteString("\n## Skills\n\n")
		for _, s := range card.Skills {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", s.Name, s.Description))
			if len(s.Tags) > 0 {
				sb.WriteString(fmt.Sprintf("  Tags: %s\n", strings.Join(s.Tags, ", ")))
			}
		}
	}

	if card.Provider != nil {
		sb.WriteString("\n## Provider\n\n")
		sb.WriteString(fmt.Sprintf("**Name:** %s\n", card.Provider.Name))
		if card.Provider.Organization != "" {
			sb.WriteString(fmt.Sprintf("**Organization:** %s\n", card.Provider.Organization))
		}
		if card.Provider.URL != "" {
			sb.WriteString(fmt.Sprintf("**URL:** %s\n", card.Provider.URL))
		}
	}

	return textResult(sb.String())
}

// ============================================================================
// Task Delegation Tools
// ============================================================================

// A2ADelegateTool delegates a task to a remote A2A agent.
type A2ADelegateTool struct {
	client *api.Client
}

func (t *A2ADelegateTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "a2a_delegate",
		Description: "Delegate a task to a remote A2A agent.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url":      map[string]string{"type": "string", "description": "Agent URL"},
				"skill_id": map[string]string{"type": "string", "description": "Skill ID to invoke"},
				"input":    map[string]string{"type": "object", "description": "Task input data"},
				"priority": map[string]string{"type": "integer", "description": "Task priority (1-10, default 5)"},
			},
			"required": []string{"url"},
		},
	}
}

func (t *A2ADelegateTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		URL      string          `json:"url"`
		SkillID  string          `json:"skill_id"`
		Input    json.RawMessage `json:"input"`
		Priority int             `json:"priority"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.URL == "" {
		return errorResult("url is required")
	}

	task, err := t.client.CreateA2ATask(ctx, params.URL, &api.CreateA2ATaskRequest{
		SkillID:  params.SkillID,
		Input:    params.Input,
		Priority: params.Priority,
	})
	if err != nil {
		return errorResult("delegating task: " + err.Error())
	}

	var sb strings.Builder
	sb.WriteString("Task delegated!\n\n")
	sb.WriteString(fmt.Sprintf("**Task ID:** %s\n", task.ID))
	sb.WriteString(fmt.Sprintf("**Status:** %s\n", task.Status))
	sb.WriteString(fmt.Sprintf("**Agent:** %s\n", task.ToAgentURL))
	if task.SkillID != "" {
		sb.WriteString(fmt.Sprintf("**Skill:** %s\n", task.SkillID))
	}
	sb.WriteString(fmt.Sprintf("\nUse `a2a_task_get` with task_id=\"%s\" to check status.", task.ID))

	return textResult(sb.String())
}

// A2ATaskGetTool gets task status.
type A2ATaskGetTool struct {
	client *api.Client
}

func (t *A2ATaskGetTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "a2a_task_get",
		Description: "Get the status of an A2A task.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url":     map[string]string{"type": "string", "description": "Agent URL"},
				"task_id": map[string]string{"type": "string", "description": "Task ID"},
			},
			"required": []string{"url", "task_id"},
		},
	}
}

func (t *A2ATaskGetTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		URL    string `json:"url"`
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.URL == "" {
		return errorResult("url is required")
	}
	if params.TaskID == "" {
		return errorResult("task_id is required")
	}

	task, err := t.client.GetA2ATask(ctx, params.URL, params.TaskID)
	if err != nil {
		return errorResult("getting task: " + err.Error())
	}

	icon := a2aStatusIcon(task.Status)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Task %s\n\n", task.ID))
	sb.WriteString(fmt.Sprintf("%s **Status:** %s\n", icon, task.Status))
	if task.StatusMessage != "" {
		sb.WriteString(fmt.Sprintf("**Message:** %s\n", task.StatusMessage))
	}
	if task.Progress != nil && task.Progress.Percentage > 0 {
		sb.WriteString(fmt.Sprintf("**Progress:** %d%% - %s\n", task.Progress.Percentage, task.Progress.CurrentStep))
	}
	sb.WriteString(fmt.Sprintf("\n**Created:** %s\n", task.CreatedAt))
	if task.StartedAt != nil {
		sb.WriteString(fmt.Sprintf("**Started:** %s\n", *task.StartedAt))
	}
	if task.CompletedAt != nil {
		sb.WriteString(fmt.Sprintf("**Completed:** %s\n", *task.CompletedAt))
	}

	if len(task.Output) > 0 {
		sb.WriteString("\n## Output\n\n")
		sb.WriteString("```json\n")
		sb.WriteString(string(task.Output))
		sb.WriteString("\n```\n")
	}

	return textResult(sb.String())
}

// A2ATaskMessageTool sends a message to a task.
type A2ATaskMessageTool struct {
	client *api.Client
}

func (t *A2ATaskMessageTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "a2a_task_message",
		Description: "Send a message to an A2A task (for tasks requiring input).",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url":     map[string]string{"type": "string", "description": "Agent URL"},
				"task_id": map[string]string{"type": "string", "description": "Task ID"},
				"text":    map[string]string{"type": "string", "description": "Text message to send"},
			},
			"required": []string{"url", "task_id", "text"},
		},
	}
}

func (t *A2ATaskMessageTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		URL    string `json:"url"`
		TaskID string `json:"task_id"`
		Text   string `json:"text"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.URL == "" {
		return errorResult("url is required")
	}
	if params.TaskID == "" {
		return errorResult("task_id is required")
	}
	if params.Text == "" {
		return errorResult("text is required")
	}

	msg, err := t.client.SendA2AMessage(ctx, params.URL, params.TaskID, &api.SendA2AMessageRequest{
		Parts: []api.A2APart{{Type: "text", Text: params.Text}},
	})
	if err != nil {
		return errorResult("sending message: " + err.Error())
	}

	return textResult(fmt.Sprintf("Message sent!\n\n**Message ID:** %s\n**Role:** %s", msg.ID, msg.Role))
}

// A2ATaskCancelTool cancels a task.
type A2ATaskCancelTool struct {
	client *api.Client
}

func (t *A2ATaskCancelTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "a2a_task_cancel",
		Description: "Cancel an A2A task.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url":     map[string]string{"type": "string", "description": "Agent URL"},
				"task_id": map[string]string{"type": "string", "description": "Task ID"},
				"reason":  map[string]string{"type": "string", "description": "Cancellation reason"},
			},
			"required": []string{"url", "task_id"},
		},
	}
}

func (t *A2ATaskCancelTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		URL    string `json:"url"`
		TaskID string `json:"task_id"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.URL == "" {
		return errorResult("url is required")
	}
	if params.TaskID == "" {
		return errorResult("task_id is required")
	}

	if err := t.client.CancelA2ATask(ctx, params.URL, params.TaskID, params.Reason); err != nil {
		return errorResult("canceling task: " + err.Error())
	}

	return textResult(fmt.Sprintf("Task %s canceled", params.TaskID))
}

// ============================================================================
// Helpers
// ============================================================================

func a2aStatusIcon(status string) string {
	switch status {
	case "completed":
		return "✅"
	case "failed":
		return "❌"
	case "running":
		return "🔄"
	case "pending":
		return "⏳"
	case "canceled":
		return "🚫"
	case "input_needed":
		return "💬"
	default:
		return "❓"
	}
}

// NewA2ATools creates all A2A tools using the given API client.
func NewA2ATools(client *api.Client) []mcp.Tool {
	return []mcp.Tool{
		// Agent registration.
		&A2AAgentsListTool{client: client},
		&A2AAgentGetTool{client: client},
		&A2AAgentCreateTool{client: client},
		&A2AAgentDeleteTool{client: client},
		// Discovery.
		&A2ADiscoverTool{client: client},
		// Task delegation.
		&A2ADelegateTool{client: client},
		&A2ATaskGetTool{client: client},
		&A2ATaskMessageTool{client: client},
		&A2ATaskCancelTool{client: client},
	}
}
