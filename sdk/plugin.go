// Package sdk provides helpers for building locksmith vault plugins.
// Plugin authors should implement the Provider interface and call Serve()
// from their main function.
package sdk

import (
	"context"
	"os/exec"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
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
	"vault": &VaultGRPCPlugin{},
}

// Serve starts the plugin gRPC server. Call this from the plugin's main().
func Serve(provider Provider) {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         map[string]goplugin.Plugin{"vault": &VaultGRPCPlugin{Impl: provider}},
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

// VaultGRPCPlugin implements goplugin.GRPCPlugin.
type VaultGRPCPlugin struct {
	goplugin.NetRPCUnsupportedPlugin
	Impl Provider
}

// NewVaultGRPCPlugin creates a VaultGRPCPlugin with the given Provider implementation.
func NewVaultGRPCPlugin(impl Provider) *VaultGRPCPlugin {
	return &VaultGRPCPlugin{Impl: impl}
}

// GRPCServer registers the VaultProvider gRPC server with the go-plugin broker.
func (p *VaultGRPCPlugin) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	vaultv1.RegisterVaultProviderServiceServer(s, NewGRPCServer(p.Impl))
	return nil
}

// GRPCClient creates a client-side adapter for the plugin.
func (p *VaultGRPCPlugin) GRPCClient(ctx context.Context, broker *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return NewGRPCClient(c), nil
}

// VaultGRPCServer is the server-side adapter that bridges go-plugin gRPC to Provider.
type VaultGRPCServer struct {
	vaultv1.UnimplementedVaultProviderServiceServer
	impl Provider
}

// NewGRPCServer wraps a Provider into a VaultProvider gRPC server.
func NewGRPCServer(impl Provider) *VaultGRPCServer {
	return &VaultGRPCServer{impl: impl}
}

// GetSecret delegates to the underlying Provider implementation.
func (s *VaultGRPCServer) GetSecret(ctx context.Context, req *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	return s.impl.GetSecret(ctx, req)
}

// HealthCheck delegates to the underlying Provider implementation.
func (s *VaultGRPCServer) HealthCheck(ctx context.Context, req *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error) {
	return s.impl.HealthCheck(ctx, req)
}

// Info delegates to the underlying Provider implementation.
func (s *VaultGRPCServer) Info(ctx context.Context, req *vaultv1.InfoRequest) (*vaultv1.InfoResponse, error) {
	return s.impl.Info(ctx, req)
}

// VaultGRPCClient is the client-side adapter used by the daemon's plugin manager.
type VaultGRPCClient struct {
	client vaultv1.VaultProviderServiceClient
}

// NewGRPCClient wraps a gRPC client connection into a Provider.
func NewGRPCClient(conn *grpc.ClientConn) Provider {
	return &VaultGRPCClient{client: vaultv1.NewVaultProviderServiceClient(conn)}
}

// GetSecret calls the remote plugin's GetSecret over gRPC.
func (c *VaultGRPCClient) GetSecret(ctx context.Context, req *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	return c.client.GetSecret(ctx, req)
}

// HealthCheck calls the remote plugin's HealthCheck over gRPC.
func (c *VaultGRPCClient) HealthCheck(ctx context.Context, req *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error) {
	return c.client.HealthCheck(ctx, req)
}

// Info calls the remote plugin's Info over gRPC.
func (c *VaultGRPCClient) Info(ctx context.Context, req *vaultv1.InfoRequest) (*vaultv1.InfoResponse, error) {
	return c.client.Info(ctx, req)
}
