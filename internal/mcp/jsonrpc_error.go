package mcp

import (
	"bytes"
	"encoding/json"
)

// rpcEnvelope is the minimal JSON-RPC 2.0 response shape needed to
// classify whether a response carries an error worth one retry.
type rpcEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
	Result  *struct {
		IsError bool `json:"isError,omitempty"`
	} `json:"result,omitempty"`
}

// inspectResponse reports whether raw looks like a JSON-RPC response that
// indicates an error worth retrying once with resolved auth headers.
// Returns false for parse failures, notifications, id:null responses,
// non-JSON-RPC payloads, and any response without an error indicator.
func inspectResponse(raw []byte) bool {
	var env rpcEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return false
	}
	if env.JSONRPC != "2.0" {
		return false
	}
	if len(env.ID) == 0 || bytes.Equal(env.ID, []byte("null")) {
		return false
	}
	if len(env.Error) > 0 {
		return true
	}
	if env.Result != nil && env.Result.IsError {
		return true
	}
	return false
}

// extractID returns the raw JSON bytes of the response's id field as a
// string, or "" if the id is missing or literal null. The string is
// suitable as a map key: two responses share the same logical id iff
// their extractID values are equal.
func extractID(raw []byte) string {
	var env rpcEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return ""
	}
	if len(env.ID) == 0 || bytes.Equal(env.ID, []byte("null")) {
		return ""
	}
	return string(env.ID)
}
