package sdk_test

import (
	"context"
	"net"
	"testing"

	sdk "github.com/lorem-dev/locksmith/sdk"
	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestVaultGRPCPlugin_GRPCServer tests that GRPCServer registers the service and
// the registered server can handle requests.
func TestVaultGRPCPlugin_GRPCServer(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcSrv := grpc.NewServer()
	plugin := sdk.NewVaultGRPCPlugin(&mockProvider{})
	if err := plugin.GRPCServer(nil, grpcSrv); err != nil {
		t.Fatalf("GRPCServer() error: %v", err)
	}
	go grpcSrv.Serve(lis) //nolint:errcheck
	defer grpcSrv.Stop()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := vaultv1.NewVaultProviderClient(conn)
	resp, err := client.GetSecret(context.Background(), &vaultv1.GetSecretRequest{Path: "test"})
	if err != nil {
		t.Fatalf("GetSecret() via registered server: %v", err)
	}
	if string(resp.Secret) != "test-secret" {
		t.Errorf("secret = %q, want %q", string(resp.Secret), "test-secret")
	}
}

// TestVaultGRPCPlugin_GRPCClient tests that GRPCClient returns a usable Provider.
func TestVaultGRPCPlugin_GRPCClient(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcSrv := grpc.NewServer()
	vaultv1.RegisterVaultProviderServer(grpcSrv, sdk.NewGRPCServer(&mockProvider{}))
	go grpcSrv.Serve(lis) //nolint:errcheck
	defer grpcSrv.Stop()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	plugin := sdk.NewVaultGRPCPlugin(&mockProvider{})
	raw, err := plugin.GRPCClient(context.Background(), nil, conn)
	if err != nil {
		t.Fatalf("GRPCClient() error: %v", err)
	}
	provider, ok := raw.(sdk.Provider)
	if !ok {
		t.Fatal("GRPCClient() did not return a Provider")
	}
	resp, err := provider.HealthCheck(context.Background(), &vaultv1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("HealthCheck() via GRPCClient provider: %v", err)
	}
	if !resp.Available {
		t.Error("Available = false, want true")
	}
}

type mockProvider struct{}

func (m *mockProvider) GetSecret(_ context.Context, _ *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	return &vaultv1.GetSecretResponse{Secret: []byte("test-secret"), ContentType: "text/plain"}, nil
}

func (m *mockProvider) HealthCheck(_ context.Context, _ *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error) {
	return &vaultv1.HealthCheckResponse{Available: true, Message: "ok"}, nil
}

func (m *mockProvider) Info(_ context.Context, _ *vaultv1.InfoRequest) (*vaultv1.PluginInfo, error) {
	return &vaultv1.PluginInfo{Name: "mock", Version: "0.1.0", Platforms: []string{"darwin"}}, nil
}

func TestGRPCServer_GetSecret(t *testing.T) {
	server := sdk.NewGRPCServer(&mockProvider{})
	if server == nil {
		t.Fatal("NewGRPCServer() returned nil")
	}
	resp, err := server.GetSecret(context.Background(), &vaultv1.GetSecretRequest{Path: "test"})
	if err != nil {
		t.Fatalf("GetSecret() error: %v", err)
	}
	if string(resp.Secret) != "test-secret" {
		t.Errorf("secret = %q, want %q", string(resp.Secret), "test-secret")
	}
}

func TestGRPCServer_HealthCheck(t *testing.T) {
	server := sdk.NewGRPCServer(&mockProvider{})
	resp, err := server.HealthCheck(context.Background(), &vaultv1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("HealthCheck() error: %v", err)
	}
	if !resp.Available {
		t.Error("Available = false, want true")
	}
}

func TestGRPCServer_Info(t *testing.T) {
	server := sdk.NewGRPCServer(&mockProvider{})
	resp, err := server.Info(context.Background(), &vaultv1.InfoRequest{})
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	if resp.Name != "mock" {
		t.Errorf("Name = %q, want %q", resp.Name, "mock")
	}
}

func TestNewClientConfig(t *testing.T) {
	cfg := sdk.NewClientConfig("/usr/local/bin/locksmith-plugin-test")
	if cfg == nil {
		t.Fatal("NewClientConfig() returned nil")
	}
}

func TestVaultGRPCClient_GetSecret(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcSrv := grpc.NewServer()
	vaultv1.RegisterVaultProviderServer(grpcSrv, sdk.NewGRPCServer(&mockProvider{}))
	go grpcSrv.Serve(lis) //nolint:errcheck
	defer grpcSrv.Stop()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := sdk.NewGRPCClient(conn)
	resp, err := client.GetSecret(context.Background(), &vaultv1.GetSecretRequest{Path: "test"})
	if err != nil {
		t.Fatalf("GetSecret() error: %v", err)
	}
	if string(resp.Secret) != "test-secret" {
		t.Errorf("secret = %q, want %q", string(resp.Secret), "test-secret")
	}
}

func TestVaultGRPCClient_HealthCheck(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcSrv := grpc.NewServer()
	vaultv1.RegisterVaultProviderServer(grpcSrv, sdk.NewGRPCServer(&mockProvider{}))
	go grpcSrv.Serve(lis) //nolint:errcheck
	defer grpcSrv.Stop()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := sdk.NewGRPCClient(conn)
	resp, err := client.HealthCheck(context.Background(), &vaultv1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("HealthCheck() error: %v", err)
	}
	if !resp.Available {
		t.Error("Available = false, want true")
	}
}

func TestVaultGRPCClient_Info(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcSrv := grpc.NewServer()
	vaultv1.RegisterVaultProviderServer(grpcSrv, sdk.NewGRPCServer(&mockProvider{}))
	go grpcSrv.Serve(lis) //nolint:errcheck
	defer grpcSrv.Stop()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := sdk.NewGRPCClient(conn)
	resp, err := client.Info(context.Background(), &vaultv1.InfoRequest{})
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	if resp.Name != "mock" {
		t.Errorf("Name = %q, want %q", resp.Name, "mock")
	}
}
