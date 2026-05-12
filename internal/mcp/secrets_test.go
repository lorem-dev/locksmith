package mcp_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lorem-dev/locksmith/internal/mcp"
)

func TestParseRef(t *testing.T) {
	tests := []struct {
		input   string
		want    mcp.SecretRef
		wantErr string
	}{
		{"github-token", mcp.SecretRef{KeyAlias: "github-token"}, ""},
		{"{key:openai-key}", mcp.SecretRef{KeyAlias: "openai-key"}, ""},
		{"{vault:gopass path:work/api/key}", mcp.SecretRef{VaultName: "gopass", Path: "work/api/key"}, ""},
		{"{vault:keychain path:org/id}", mcp.SecretRef{VaultName: "keychain", Path: "org/id"}, ""},
		{"{key:}", mcp.SecretRef{}, "key alias cannot be empty"},
		{"{vault:gopass}", mcp.SecretRef{}, "vault reference requires both 'vault' and 'path'"},
		{"{path:foo}", mcp.SecretRef{}, "vault reference requires both 'vault' and 'path'"},
		{"{unknown:x}", mcp.SecretRef{}, "unknown reference field"},
		{"{unclosed", mcp.SecretRef{}, "unclosed {"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := mcp.ParseRef(tc.input)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestResolveTemplate(t *testing.T) {
	fetcher := staticFetcher{
		"openai-key":      "sk-123",
		"gopass:work/api": "vault-secret",
	}
	tests := []struct {
		tmpl string
		want string
	}{
		{"Bearer {key:openai-key}", "Bearer sk-123"},
		{"{vault:gopass path:work/api}", "vault-secret"},
		{"no tokens here", "no tokens here"},
		{"prefix {key:openai-key} suffix", "prefix sk-123 suffix"},
	}
	for _, tc := range tests {
		t.Run(tc.tmpl, func(t *testing.T) {
			got, err := mcp.ResolveTemplate(context.Background(), tc.tmpl, fetcher)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// staticFetcher is a test double for SecretFetcher.
type staticFetcher map[string]string

func (f staticFetcher) Fetch(_ context.Context, ref mcp.SecretRef) (string, error) {
	if ref.KeyAlias != "" {
		if v, ok := f[ref.KeyAlias]; ok {
			return v, nil
		}
		return "", fmt.Errorf("key %q not found", ref.KeyAlias)
	}
	k := ref.VaultName + ":" + ref.Path
	if v, ok := f[k]; ok {
		return v, nil
	}
	return "", fmt.Errorf("vault %q path %q not found", ref.VaultName, ref.Path)
}
