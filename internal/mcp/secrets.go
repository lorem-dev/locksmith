// Package mcp implements the locksmith mcp run subcommand: secret injection
// for local MCP server processes and stdio<->HTTP proxy for remote servers.
package mcp

import (
	"context"
	"fmt"
	"strings"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
)

// SecretRef describes how to retrieve a secret from the daemon.
// Exactly one of KeyAlias or (VaultName + Path) must be set.
type SecretRef struct {
	KeyAlias  string
	VaultName string
	Path      string
}

// SecretFetcher retrieves a secret by reference.
type SecretFetcher interface {
	Fetch(ctx context.Context, ref SecretRef) (string, error)
}

// GRPCFetcher retrieves secrets via the locksmith gRPC daemon.
type GRPCFetcher struct {
	Client    locksmithv1.LocksmithServiceClient
	SessionID string
}

// Fetch implements SecretFetcher.
func (f *GRPCFetcher) Fetch(ctx context.Context, ref SecretRef) (string, error) {
	resp, err := f.Client.GetSecret(ctx, &locksmithv1.GetSecretRequest{
		SessionId: f.SessionID,
		KeyAlias:  ref.KeyAlias,
		VaultName: ref.VaultName,
		Path:      ref.Path,
	})
	if err != nil {
		return "", fmt.Errorf("getting secret: %w", err)
	}
	return string(resp.Secret), nil
}

// ParseRef parses a secret reference string.
//
//   - Bare string (no braces): treated as key alias shorthand.
//   - {key:alias}: explicit key alias.
//   - {vault:name path:value}: direct vault + path lookup.
func ParseRef(s string) (SecretRef, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") {
		if s == "" {
			return SecretRef{}, fmt.Errorf("secret reference cannot be empty")
		}
		return SecretRef{KeyAlias: s}, nil
	}
	if !strings.HasSuffix(s, "}") {
		return SecretRef{}, fmt.Errorf("unclosed { in reference %q", s)
	}
	return parseInnerRef(s[1 : len(s)-1])
}

func parseInnerRef(inner string) (SecretRef, error) {
	fields := strings.Fields(inner)
	var ref SecretRef
	for _, field := range fields {
		k, v, ok := strings.Cut(field, ":")
		if !ok {
			return SecretRef{}, fmt.Errorf("invalid reference field %q: expected key:value", field)
		}
		switch k {
		case "key":
			if v == "" {
				return SecretRef{}, fmt.Errorf("key alias cannot be empty")
			}
			ref.KeyAlias = v
		case "vault":
			ref.VaultName = v
		case "path":
			ref.Path = v
		default:
			return SecretRef{}, fmt.Errorf("unknown reference field %q", k)
		}
	}
	if ref.KeyAlias == "" && (ref.VaultName == "" || ref.Path == "") {
		return SecretRef{}, fmt.Errorf("vault reference requires both 'vault' and 'path' fields")
	}
	return ref, nil
}

// ResolveTemplate resolves all {key:alias} and {vault:name path:value} tokens
// in a template string, replacing each with the fetched secret value.
func ResolveTemplate(ctx context.Context, tmpl string, fetcher SecretFetcher) (string, error) {
	var result strings.Builder
	remaining := tmpl
	for {
		start := strings.IndexByte(remaining, '{')
		if start == -1 {
			result.WriteString(remaining)
			break
		}
		result.WriteString(remaining[:start])
		end := strings.IndexByte(remaining[start:], '}')
		if end == -1 {
			return "", fmt.Errorf("unclosed { in template %q", tmpl)
		}
		end += start
		ref, err := parseInnerRef(remaining[start+1 : end])
		if err != nil {
			return "", fmt.Errorf("invalid reference in %q: %w", tmpl, err)
		}
		value, err := fetcher.Fetch(ctx, ref)
		if err != nil {
			return "", fmt.Errorf("resolving reference in %q: %w", tmpl, err)
		}
		result.WriteString(value)
		remaining = remaining[end+1:]
	}
	return result.String(), nil
}
