package mcp_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lorem-dev/locksmith/internal/mcp"
)

func TestParseSSE(t *testing.T) {
	input := "event: endpoint\ndata: /messages\n\nevent: message\ndata: {\"jsonrpc\":\"2.0\"}\n\n"
	events := mcp.CollectSSE(strings.NewReader(input))
	require.Len(t, events, 2)
	assert.Equal(t, "endpoint", events[0].Type)
	assert.Equal(t, "/messages", events[0].Data)
	assert.Equal(t, "message", events[1].Type)
	assert.Equal(t, `{"jsonrpc":"2.0"}`, events[1].Data)
}

func TestParseSSE_DataOnly(t *testing.T) {
	input := "data: hello\n\ndata: world\n\n"
	events := mcp.CollectSSE(strings.NewReader(input))
	require.Len(t, events, 2)
	assert.Equal(t, "", events[0].Type)
	assert.Equal(t, "hello", events[0].Data)
}

func TestNewTransport_InvalidType(t *testing.T) {
	_, err := mcp.NewTransport("https://example.com", nil, "grpc")
	require.ErrorContains(t, err, "unknown transport")
}
