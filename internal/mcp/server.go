// Package mcp exposes odotoken's read-only analytics to an MCP client over
// stdio. The contract is deliberately the three stable primitives (initialize,
// tools/list, tools/call) instead of a vendor SDK: an MCP client that speaks
// JSON-RPC 2.0 can use it, and there is no second HTTP surface to authenticate.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
)

// ProtocolVersion is the MCP revision this server implements.
const ProtocolVersion = "2024-11-05"

// Tool is one callable capability.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Request is one JSON-RPC 2.0 message from the client.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is one JSON-RPC 2.0 reply. A notification (no ID) produces none.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is the JSON-RPC error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToolResult is the MCP content envelope for a successful tool call. Values are
// JSON text because MCP content is a list of typed blocks, not raw JSON.
type ToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Content is one MCP content block.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// CallFunc answers one tool invocation. Returning an error is a tool failure
// (isError true), not a protocol failure: the model can read and recover.
type CallFunc func(args map[string]interface{}) (interface{}, error)

// Server holds the tool table and answers requests. It is safe for concurrent
// use even though the stdio loop is sequential, so a test can drive it directly.
type Server struct {
	mu    sync.RWMutex
	tools []Tool
	calls map[string]CallFunc
}

// NewServer builds a server with the given tools.
func NewServer(tools []Tool, calls map[string]CallFunc) *Server {
	return &Server{tools: tools, calls: calls}
}

// Tools returns a copy of the tool list, ordered by name for a stable listing.
func (s *Server) Tools() []Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Tool, len(s.tools))
	copy(out, s.tools)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Handle answers one request. It never returns an error: protocol failures are
// encoded in Response.Error so the transport stays alive.
func (s *Server) Handle(req Request) *Response {
	if req.JSONRPC != "" && req.JSONRPC != "2.0" {
		return &Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32600, Message: "invalid jsonrpc version"}}
	}
	resp := &Response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = map[string]interface{}{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo":      map[string]interface{}{"name": "odotoken", "version": "1"},
		}
	case "notifications/initialized", "notifications/cancelled":
		if len(req.ID) == 0 {
			return nil
		}
		resp.Result = map[string]interface{}{}
	case "ping":
		resp.Result = map[string]interface{}{}
	case "tools/list":
		resp.Result = map[string]interface{}{"tools": s.Tools()}
	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = &RPCError{Code: -32602, Message: "invalid tools/call params"}
			return resp
		}
		s.mu.RLock()
		call, ok := s.calls[params.Name]
		s.mu.RUnlock()
		if !ok {
			resp.Error = &RPCError{Code: -32602, Message: fmt.Sprintf("unknown tool %q", params.Name)}
			return resp
		}
		value, err := call(params.Arguments)
		if err != nil {
			resp.Result = ToolResult{IsError: true, Content: []Content{{Type: "text", Text: err.Error()}}}
			return resp
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			resp.Result = ToolResult{IsError: true, Content: []Content{{Type: "text", Text: "result is not JSON-serializable"}}}
			return resp
		}
		resp.Result = ToolResult{Content: []Content{{Type: "text", Text: string(encoded)}}}
	default:
		resp.Error = &RPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)}
	}
	return resp
}

// Serve reads newline-delimited JSON-RPC messages until r ends. A malformed
// line is answered with a parse error and does not stop the stream, because a
// client that sends one bad frame should not lose the session.
func (s *Server) Serve(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	enc := json.NewEncoder(w)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			if err := enc.Encode(Response{JSONRPC: "2.0", Error: &RPCError{Code: -32700, Message: "parse error"}}); err != nil {
				return err
			}
			continue
		}
		resp := s.Handle(req)
		if resp == nil {
			continue
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// StringArg reads an optional string argument. MCP clients send numbers as
// JSON numbers, so a numeric value is stringified instead of rejected.
func StringArg(args map[string]interface{}, key string) string {
	switch value := args[key].(type) {
	case string:
		return value
	case float64:
		if value == math.Trunc(value) {
			return fmt.Sprintf("%.0f", value)
		}
		return fmt.Sprintf("%v", value)
	default:
		return ""
	}
}

// IntArg reads an optional integer argument, ignoring a non-numeric value.
func IntArg(args map[string]interface{}, key string, fallback int) int {
	switch value := args[key].(type) {
	case float64:
		return int(value)
	case string:
		var n int
		if _, err := fmt.Sscanf(value, "%d", &n); err == nil {
			return n
		}
	}
	return fallback
}
