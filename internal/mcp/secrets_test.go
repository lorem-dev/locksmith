package mcp_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
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

// fakeLocksmithClient is a minimal stub for locksmithv1.LocksmithServiceClient
// that returns canned responses for GetSecret. Methods not overridden
// inherit nil implementations from the embedded interface and panic if
// invoked, which is fine for the GRPCFetcher tests that only exercise
// GetSecret.
type fakeLocksmithClient struct {
	locksmithv1.LocksmithServiceClient // embed interface for unimplemented methods

	getSecretFn func(req *locksmithv1.GetSecretRequest) (*locksmithv1.GetSecretResponse, error)
}

func (c *fakeLocksmithClient) GetSecret(
	_ context.Context,
	req *locksmithv1.GetSecretRequest,
	_ ...grpc.CallOption,
) (*locksmithv1.GetSecretResponse, error) {
	return c.getSecretFn(req)
}

func TestGRPCFetcher_RefreshOnExpiredSession(t *testing.T) {
	var calls int
	client := &fakeLocksmithClient{
		getSecretFn: func(req *locksmithv1.GetSecretRequest) (*locksmithv1.GetSecretResponse, error) {
			calls++
			if calls == 1 {
				require.Equal(t, "old-session", req.SessionId)
				return nil, fmt.Errorf("invalid session: session for prefix %q not found", "ls_old")
			}
			require.Equal(t, "new-session", req.SessionId)
			return &locksmithv1.GetSecretResponse{Secret: []byte("the-secret")}, nil
		},
	}
	var refreshCalls int
	fetcher := &mcp.GRPCFetcher{
		Client:    client,
		SessionID: "old-session",
		RefreshSession: func(_ context.Context) (string, error) {
			refreshCalls++
			return "new-session", nil
		},
	}

	got, err := fetcher.Fetch(context.Background(), mcp.SecretRef{KeyAlias: "k"})
	require.NoError(t, err)
	assert.Equal(t, "the-secret", got)
	assert.Equal(t, 2, calls)
	assert.Equal(t, 1, refreshCalls)
	assert.Equal(t, "new-session", fetcher.SessionID)
}

func TestGRPCFetcher_NoRefreshWhenRefresherNil(t *testing.T) {
	client := &fakeLocksmithClient{
		getSecretFn: func(_ *locksmithv1.GetSecretRequest) (*locksmithv1.GetSecretResponse, error) {
			return nil, fmt.Errorf("invalid session: missing")
		},
	}
	fetcher := &mcp.GRPCFetcher{
		Client:    client,
		SessionID: "old-session",
		// RefreshSession deliberately nil
	}
	_, err := fetcher.Fetch(context.Background(), mcp.SecretRef{KeyAlias: "k"})
	require.ErrorContains(t, err, "invalid session")
	assert.Equal(t, "old-session", fetcher.SessionID)
}

func TestGRPCFetcher_NoRetryOnNonExpiryError(t *testing.T) {
	var calls int
	client := &fakeLocksmithClient{
		getSecretFn: func(_ *locksmithv1.GetSecretRequest) (*locksmithv1.GetSecretResponse, error) {
			calls++
			return nil, fmt.Errorf("vault unreachable")
		},
	}
	var refreshCalls int
	fetcher := &mcp.GRPCFetcher{
		Client:    client,
		SessionID: "old-session",
		RefreshSession: func(_ context.Context) (string, error) {
			refreshCalls++
			return "new-session", nil
		},
	}
	_, err := fetcher.Fetch(context.Background(), mcp.SecretRef{KeyAlias: "k"})
	require.ErrorContains(t, err, "vault unreachable")
	assert.Equal(t, 1, calls)
	assert.Equal(t, 0, refreshCalls)
}
