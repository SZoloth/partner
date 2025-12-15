package mcp

import (
	"context"
	"encoding/json"
)

// ToolResult represents the result of an MCP tool call
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in the tool result
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Transport abstracts stdio vs HTTP connections
type Transport interface {
	Call(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
	Close() error
}

// Client provides a unified interface to MCP servers
type Client struct {
	transport Transport
	serverID  string
}

// NewClient creates a new MCP client
func NewClient(transport Transport, serverID string) *Client {
	return &Client{
		transport: transport,
		serverID:  serverID,
	}
}

// CallTool invokes an MCP tool
func (c *Client) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (*ToolResult, error) {
	params := map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	}

	result, err := c.transport.Call(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}

	var toolResult ToolResult
	if err := json.Unmarshal(result, &toolResult); err != nil {
		return nil, err
	}

	return &toolResult, nil
}

// ListTools returns available tools from the server
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	result, err := c.transport.Call(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, err
	}

	return response.Tools, nil
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Close closes the client's transport
func (c *Client) Close() error {
	return c.transport.Close()
}
