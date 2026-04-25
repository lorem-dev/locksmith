// Package vault provides the Provider interface and gRPC adapters for building
// locksmith vault plugins. Plugin authors implement Provider and call Serve().
package vault

import (
	"context"
	"os/exec"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
	sdkerrors "github.com/lorem-dev/locksmith/sdk/errors"
)

// Provider is the interface that vault plugin binaries must implement.
// Each method is called by the daemon over gRPC.
type Provider interface {
	GetSecret(ctx context.Context, req *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error)
	HealthCheck(ctx context.Context, req *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error)
	Info(ctx context.Context, req *vaultv1.InfoRequest) (*vaultv1.InfoResponse, error)
}

// Handshake is the go-plugin handshake config. Both the daemon and every plugin
// binary must use the same values for the connection to be established.
var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "LOCKSMITH_PLUGIN",
	MagicCookieValue: "vault-provider",
}

// PluginMap is the map passed to go-plugin. Key is the plugin type name.
var PluginMap = map[string]goplugin.Plugin{
	"vault": &GRPCPlugin{},
}

// Serve starts the plugin gRPC server. Call this from the plugin's main().
func Serve(provider Provider) {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         map[string]goplugin.Plugin{"vault": &GRPCPlugin{Impl: provider}},
		GRPCServer:      goplugin.DefaultGRPCServer,
	})
}

// NewClientConfig returns a go-plugin ClientConfig for a given plugin binary path.
func NewClientConfig(binaryPath string) *goplugin.ClientConfig {
	return &goplugin.ClientConfig{
		HandshakeConfig:  Handshake,
		Plugins:          PluginMap,
		Cmd:              exec.Command(binaryPath),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
	}
}

// GRPCPlugin implements goplugin.GRPCPlugin.
type GRPCPlugin struct {
	goplugin.NetRPCUnsupportedPlugin
	Impl Provider
}

// NewGRPCPlugin creates a GRPCPlugin with the given Provider implementation.
func NewGRPCPlugin(impl Provider) *GRPCPlugin {
	return &GRPCPlugin{Impl: impl}
}

// GRPCServer registers the VaultProvider gRPC server with the go-plugin broker.
func (p *GRPCPlugin) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	vaultv1.RegisterVaultProviderServiceServer(s, NewGRPCServer(p.Impl))
	return nil
}

// GRPCClient creates a client-side adapter for the plugin.
func (p *GRPCPlugin) GRPCClient(
	ctx context.Context,
	broker *goplugin.GRPCBroker,
	c *grpc.ClientConn,
) (interface{}, error) {
	return NewGRPCClient(c), nil
}

// GRPCServer is the server-side adapter that bridges go-plugin gRPC to Provider.
type GRPCServer struct {
	vaultv1.UnimplementedVaultProviderServiceServer
	impl Provider
}

// NewGRPCServer wraps a Provider into a VaultProvider gRPC server.
func NewGRPCServer(impl Provider) *GRPCServer {
	return &GRPCServer{impl: impl}
}

// GetSecret delegates to the underlying Provider implementation.
func (s *GRPCServer) GetSecret(ctx context.Context, req *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	return s.impl.GetSecret(ctx, req)
}

// HealthCheck delegates to the underlying Provider implementation.
func (s *GRPCServer) HealthCheck(
	ctx context.Context,
	req *vaultv1.HealthCheckRequest,
) (*vaultv1.HealthCheckResponse, error) {
	return s.impl.HealthCheck(ctx, req)
}

// Info delegates to the underlying Provider implementation.
func (s *GRPCServer) Info(ctx context.Context, req *vaultv1.InfoRequest) (*vaultv1.InfoResponse, error) {
	return s.impl.Info(ctx, req)
}

// GRPCClient is the client-side adapter used by the daemon's plugin manager.
type GRPCClient struct {
	client vaultv1.VaultProviderServiceClient
}

// NewGRPCClient wraps a gRPC client connection into a Provider.
func NewGRPCClient(conn *grpc.ClientConn) Provider {
	return &GRPCClient{client: vaultv1.NewVaultProviderServiceClient(conn)}
}

// GetSecret calls the remote plugin's GetSecret over gRPC and unwraps any
// gRPC status error into a *VaultError so the status code survives the
// second gRPC boundary (plugin -> daemon -> CLI).
func (c *GRPCClient) GetSecret(ctx context.Context, req *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	resp, err := c.client.GetSecret(ctx, req)
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() != codes.OK {
			return nil, &sdkerrors.VaultError{Code: s.Code(), Message: s.Message()}
		}
		return nil, err
	}
	return resp, nil
}

// HealthCheck calls the remote plugin's HealthCheck over gRPC.
func (c *GRPCClient) HealthCheck(
	ctx context.Context,
	req *vaultv1.HealthCheckRequest,
) (*vaultv1.HealthCheckResponse, error) {
	return c.client.HealthCheck(ctx, req)
}

// Info calls the remote plugin's Info over gRPC.
func (c *GRPCClient) Info(ctx context.Context, req *vaultv1.InfoRequest) (*vaultv1.InfoResponse, error) {
	return c.client.Info(ctx, req)
}
