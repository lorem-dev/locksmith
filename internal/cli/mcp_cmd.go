package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/mcp"
)

// newMCPCmd returns the `locksmith mcp` command group.
func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server wrapper with secret injection",
	}
	cmd.AddCommand(newMCPRunCmd())
	return cmd
}

// newMCPRunCmd returns the `locksmith mcp run` command.
func newMCPRunCmd() *cobra.Command {
	var (
		serverName string
		envArgs    []string
		headerArgs []string
		urlArg     string
		transport  string
	)

	cmd := &cobra.Command{
		Use:   "run [-- command [args...]]",
		Short: "Run or proxy an MCP server with secrets injected",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPRun(serverName, envArgs, headerArgs, urlArg, transport, args)
		},
	}

	cmd.Flags().StringVar(&serverName, "server", "", "named server from mcp.servers in config.yaml")
	cmd.Flags().StringArrayVar(&envArgs, "env", nil, "env injection: VAR=alias or VAR={key:alias}")
	cmd.Flags().StringArrayVar(&headerArgs, "header", nil, "header injection: Name=template")
	cmd.Flags().StringVar(&urlArg, "url", "", "remote MCP server URL (proxy mode)")
	cmd.Flags().StringVar(&transport, "transport", "auto", "HTTP transport: auto|sse|http")

	return cmd
}

const mcpInitTimeout = 30 * time.Second

func runMCPRun(serverName string, envArgs, headerArgs []string, urlArg, transport string, args []string) error {
	hasURL := urlArg != ""
	hasCommand := len(args) > 0
	hasServer := serverName != ""

	if err := validateMCPRunFlags(hasURL, hasCommand, hasServer, envArgs, headerArgs); err != nil {
		return err
	}
	if !hasServer {
		if hasURL {
			if _, err := parseHeaderArgs(headerArgs); err != nil {
				return err
			}
		}
		if hasCommand {
			if _, err := parseEnvArgs(envArgs); err != nil {
				return err
			}
		}
	}

	initCtx, initCancel := context.WithTimeout(context.Background(), mcpInitTimeout)
	defer initCancel()

	client, conn, err := dialDaemon()
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // gRPC connection close error not actionable

	sessionID, err := ensureMCPSession(initCtx, client)
	if err != nil {
		return err
	}
	fetcher := &mcp.GRPCFetcher{Client: client, SessionID: sessionID}

	runCtx := context.Background()
	if hasServer {
		return runFromConfig(runCtx, fetcher, serverName)
	}
	if hasURL {
		return runProxyMode(runCtx, fetcher, urlArg, headerArgs, transport)
	}
	return runLocalMode(runCtx, fetcher, envArgs, args)
}

func validateMCPRunFlags(hasURL, hasCommand, hasServer bool, envArgs, headerArgs []string) error {
	hasInlineFlags := len(envArgs) > 0 || len(headerArgs) > 0 || hasURL || hasCommand
	if hasURL && hasCommand {
		return fmt.Errorf("--url and -- are mutually exclusive: use --url for proxy mode or -- for local mode")
	}
	if hasServer && hasInlineFlags {
		return fmt.Errorf("--server cannot be combined with --env, --header, --url, or a command")
	}
	if !hasURL && !hasCommand && !hasServer {
		return fmt.Errorf("specify --url, --server, or a command after --")
	}
	return nil
}

func ensureMCPSession(ctx context.Context, client locksmithv1.LocksmithServiceClient) (string, error) {
	existing := os.Getenv("LOCKSMITH_SESSION")
	if existing != "" {
		listResp, err := client.SessionList(ctx, &locksmithv1.SessionListRequest{})
		if err == nil {
			for _, s := range listResp.Sessions {
				if s.SessionId == existing {
					return existing, nil
				}
			}
		}
	}
	resp, err := client.SessionStart(ctx, &locksmithv1.SessionStartRequest{})
	if err != nil {
		return "", fmt.Errorf("starting session: %w", err)
	}
	return resp.SessionId, nil
}

func runLocalMode(ctx context.Context, fetcher mcp.SecretFetcher, envArgs, command []string) error {
	mappings, err := parseEnvArgs(envArgs)
	if err != nil {
		return err
	}
	return mcp.Run(ctx, fetcher, mappings, command)
}

func runProxyMode(
	ctx context.Context,
	fetcher mcp.SecretFetcher,
	url string,
	headerArgs []string,
	transport string,
) error {
	headers, err := parseHeaderArgs(headerArgs)
	if err != nil {
		return err
	}
	cfg := mcp.ProxyConfig{
		URL:       url,
		Transport: transport,
		Headers:   headers,
	}
	return mcp.RunProxy(ctx, fetcher, cfg, os.Stdin, os.Stdout)
}

func runFromConfig(ctx context.Context, fetcher mcp.SecretFetcher, serverName string) error {
	cfgPath := config.DefaultConfigPath()
	server, err := mcp.LoadServerConfig(cfgPath, serverName)
	if err != nil {
		return err
	}
	if server.URL != "" {
		headers := make([]mcp.HeaderMapping, 0, len(server.Headers))
		for name, tmpl := range server.Headers {
			headers = append(headers, mcp.HeaderMapping{Name: name, Template: tmpl})
		}
		transport := server.Transport
		if transport == "" {
			transport = "auto"
		}
		return mcp.RunProxy(ctx, fetcher, mcp.ProxyConfig{
			URL:       server.URL,
			Transport: transport,
			Headers:   headers,
		}, os.Stdin, os.Stdout)
	}
	mappings := make([]mcp.EnvMapping, 0, len(server.Env))
	for varName, ev := range server.Env {
		ref := mcp.SecretRef{
			KeyAlias:  ev.KeyAlias,
			VaultName: ev.VaultName,
			Path:      ev.Path,
		}
		mappings = append(mappings, mcp.EnvMapping{Var: varName, Ref: ref})
	}
	return mcp.Run(ctx, fetcher, mappings, server.Command)
}

func parseEnvArgs(args []string) ([]mcp.EnvMapping, error) {
	mappings := make([]mcp.EnvMapping, 0, len(args))
	for _, arg := range args {
		idx := strings.IndexByte(arg, '=')
		if idx < 1 {
			return nil, fmt.Errorf("--env %q: expected VAR=ref", arg)
		}
		ref, err := mcp.ParseRef(arg[idx+1:])
		if err != nil {
			return nil, fmt.Errorf("--env %q: %w", arg, err)
		}
		mappings = append(mappings, mcp.EnvMapping{Var: arg[:idx], Ref: ref})
	}
	return mappings, nil
}

func parseHeaderArgs(args []string) ([]mcp.HeaderMapping, error) {
	mappings := make([]mcp.HeaderMapping, 0, len(args))
	for _, arg := range args {
		idx := strings.IndexByte(arg, '=')
		if idx < 1 {
			return nil, fmt.Errorf("--header %q: expected Name=template", arg)
		}
		mappings = append(mappings, mcp.HeaderMapping{
			Name:     arg[:idx],
			Template: arg[idx+1:],
		})
	}
	return mappings, nil
}
