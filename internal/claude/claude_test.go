package claude

import (
	"slices"
	"testing"
)

func flagValue(args []string, flag string) (string, bool) {
	i := slices.Index(args, flag)
	if i < 0 || i+1 >= len(args) {
		return "", false
	}
	return args[i+1], true
}

func TestBuildArgsDisablesToolsByDefault(t *testing.T) {
	args := buildArgs("hi", Request{}, "")

	if v, ok := flagValue(args, "--tools"); !ok || v != "" {
		t.Fatalf("tools not disabled: %q", args)
	}
	if !slices.Contains(args, "--strict-mcp-config") {
		t.Fatalf("MCP servers not restricted: %q", args)
	}
	if v, ok := flagValue(args, "--mcp-config"); !ok || v != noMCPConfig {
		t.Fatalf("empty MCP config missing: %q", args)
	}
}

func TestBuildArgsKeepsToolsWhenAllowed(t *testing.T) {
	args := buildArgs("hi", Request{AllowTools: true}, "")

	if slices.Contains(args, "--tools") || slices.Contains(args, "--strict-mcp-config") {
		t.Fatalf("tools restricted although allowed: %q", args)
	}
}

func TestBuildArgsSession(t *testing.T) {
	if v, ok := flagValue(buildArgs("hi", Request{}, "abc"), "--session-id"); !ok || v != "abc" {
		t.Fatal("existing session not reused")
	}
	if slices.Contains(buildArgs("hi", Request{NewSession: true}, "abc"), "--session-id") {
		t.Fatal("new session request still reused the session")
	}
}
