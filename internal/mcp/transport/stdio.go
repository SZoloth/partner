package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
)

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCNotification represents a JSON-RPC 2.0 notification (no id field)
type JSONRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// StdioTransport communicates with MCP servers via stdio
type StdioTransport struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    *bufio.Reader
	stderr    io.ReadCloser
	mu        sync.Mutex
	reqID     int64
	started   bool
	startOnce sync.Once
}

// StdioOption configures a StdioTransport
type StdioOption func(*exec.Cmd)

// WithEnv adds environment variables to the command
func WithEnv(env ...string) StdioOption {
	return func(cmd *exec.Cmd) {
		cmd.Env = append(cmd.Environ(), env...)
	}
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport(command string, args []string, opts ...StdioOption) (*StdioTransport, error) {
	cmd := exec.Command(command, args...)
	for _, opt := range opts {
		opt(cmd)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	return &StdioTransport{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
		stderr: stderr,
	}, nil
}

// Start starts the MCP server process
func (t *StdioTransport) Start() error {
	var startErr error
	t.startOnce.Do(func() {
		if err := t.cmd.Start(); err != nil {
			startErr = fmt.Errorf("failed to start MCP server: %w", err)
			return
		}
		t.started = true

		// Drain stderr in background to prevent blocking
		go func() {
			io.Copy(io.Discard, t.stderr)
		}()

		// Initialize the connection
		if err := t.initialize(); err != nil {
			startErr = fmt.Errorf("failed to initialize MCP connection: %w", err)
			return
		}
	})
	return startErr
}

// initialize sends the MCP initialization handshake
func (t *StdioTransport) initialize() error {
	ctx := context.Background()

	// Send initialize request
	initParams := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "partner",
			"version": "0.1.0",
		},
	}

	_, err := t.Call(ctx, "initialize", initParams)
	if err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	// Send initialized notification (must NOT have id field)
	notif := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	data, _ := json.Marshal(notif)
	t.mu.Lock()
	_, err = t.stdin.Write(append(data, '\n'))
	t.mu.Unlock()

	return err
}

// Call makes a JSON-RPC call to the MCP server
func (t *StdioTransport) Call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	// Ensure started
	if !t.started {
		if err := t.Start(); err != nil {
			return nil, err
		}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	id := atomic.AddInt64(&t.reqID, 1)
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// Send request
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if _, err := t.stdin.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Read response (may need to skip notifications)
	for {
		line, err := t.stdout.ReadBytes('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		var resp JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			// Skip malformed lines
			continue
		}

		// Skip notifications (no ID)
		if resp.ID == 0 && resp.Result == nil && resp.Error == nil {
			continue
		}

		// Check for matching response
		if resp.ID == id {
			if resp.Error != nil {
				return nil, resp.Error
			}
			return resp.Result, nil
		}
	}
}

// Close terminates the MCP server process
func (t *StdioTransport) Close() error {
	if t.cmd != nil && t.cmd.Process != nil {
		t.stdin.Close()
		return t.cmd.Process.Kill()
	}
	return nil
}
