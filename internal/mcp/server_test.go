package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ens-Emilio/odotoken/internal/domain"
)

// A client that knows only initialize/tools/list/tools/call must be able to use
// the stdio server, and an unknown method must be an error object rather than a
// dropped connection.
func TestServerSpeaksMinimalMCP(t *testing.T) {
	server := NewServer(
		[]Tool{{Name: "echo", Description: "echo", InputSchema: map[string]interface{}{"type": "object"}}},
		map[string]CallFunc{
			"echo": func(args map[string]interface{}) (interface{}, error) {
				return map[string]interface{}{"got": args["value"]}, nil
			},
		},
	)

	requests := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"value":"hi"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"nope"}`,
		`not json`,
	}, "\n")

	var out strings.Builder
	if err := server.Serve(strings.NewReader(requests), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 responses (notification is silent), got %d: %s", len(lines), out.String())
	}

	responses := make([]Response, len(lines))
	for i, line := range lines {
		if err := json.Unmarshal([]byte(line), &responses[i]); err != nil {
			t.Fatalf("response %d is not JSON: %v (%s)", i, err, line)
		}
	}

	if responses[0].Error != nil {
		t.Fatalf("initialize failed: %+v", responses[0].Error)
	}
	init, _ := responses[0].Result.(map[string]interface{})
	if init["protocolVersion"] != ProtocolVersion {
		t.Fatalf("protocol version missing: %+v", init)
	}

	tools, _ := responses[1].Result.(map[string]interface{})["tools"].([]interface{})
	if len(tools) != 1 {
		t.Fatalf("unexpected tool list: %+v", responses[1].Result)
	}

	call, _ := responses[2].Result.(map[string]interface{})
	content, _ := call["content"].([]interface{})
	if len(content) != 1 {
		t.Fatalf("unexpected call result: %+v", responses[2].Result)
	}
	block, _ := content[0].(map[string]interface{})
	if block["text"] != `{"got":"hi"}` {
		t.Fatalf("unexpected content: %+v", block)
	}

	if responses[3].Error == nil {
		t.Fatal("unknown method must be an error")
	}
	if responses[4].Error == nil || responses[4].Error.Code != -32700 {
		t.Fatalf("malformed line must be a parse error, got %+v", responses[4].Error)
	}
}

func TestGuardToolReportsMissingGuard(t *testing.T) {
	server := NewOdotokenServer(nil, nil, nil)
	resp := callTool(t, server, "odotoken_guard", `{}`)
	if resp.Error != nil {
		t.Fatalf("protocol error: %+v", resp.Error)
	}
	result, _ := resp.Result.(map[string]interface{})
	if result["isError"] != true {
		t.Fatalf("a nil guard must not answer with a fake status: %+v", result)
	}
}

func TestGuardToolPassesProjectThrough(t *testing.T) {
	var seen string
	server := NewOdotokenServer(nil, func(project string) domain.GuardCheckResult {
		seen = project
		return domain.GuardCheckResult{Status: "ok", Allowed: true}
	}, nil)
	resp := callTool(t, server, "odotoken_guard", `{"project":"acme"}`)
	if resp.Error != nil {
		t.Fatalf("guard call failed: %+v", resp.Error)
	}
	if seen != "acme" {
		t.Fatalf("project was not forwarded: %q", seen)
	}
}

func TestForecastToolRejectsOutOfRangeDays(t *testing.T) {
	server := NewOdotokenServer(nil, nil, nil)
	resp := callTool(t, server, "odotoken_forecast", `{"days":91}`)
	result, _ := resp.Result.(map[string]interface{})
	if result["isError"] != true {
		t.Fatalf("days above the horizon must be rejected: %+v", result)
	}
}

// callTool sends one tools/call frame through the real transport and returns
// the decoded response, so the test exercises the same path a client does.
func callTool(t *testing.T, server *Server, name, args string) Response {
	t.Helper()
	frame := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + name + `","arguments":` + args + `}}`
	var out strings.Builder
	if err := server.Serve(strings.NewReader(frame), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	var resp Response
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &resp); err != nil {
		t.Fatalf("decode: %v (%s)", err, out.String())
	}
	return resp
}
