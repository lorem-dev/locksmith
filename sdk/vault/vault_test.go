package vault_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
	sdkerrors "github.com/lorem-dev/locksmith/sdk/errors"
	"github.com/lorem-dev/locksmith/sdk/vault"
)

// mockProvider is a Provider that returns fixed responses.
type mockProvider struct{}

func (m *mockProvider) GetSecret(_ context.Context, _ *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	return &vaultv1.GetSecretResponse{Secret: []byte("test-secret"), ContentType: "text/plain"}, nil
}

func (m *mockProvider) SetSecret(_ context.Context, _ *vaultv1.SetSecretRequest) (*vaultv1.SetSecretResponse, error) {
	return &vaultv1.SetSecretResponse{}, nil
}

func (m *mockProvider) KeyExists(_ context.Context, _ *vaultv1.KeyExistsRequest) (*vaultv1.KeyExistsResponse, error) {
	return &vaultv1.KeyExistsResponse{}, nil
}

func (m *mockProvider) HealthCheck(
	_ context.Context,
	_ *vaultv1.HealthCheckRequest,
) (*vaultv1.HealthCheckResponse, error) {
	return &vaultv1.HealthCheckResponse{Available: true, Message: "ok"}, nil
}

func (m *mockProvider) Info(_ context.Context, _ *vaultv1.InfoRequest) (*vaultv1.InfoResponse, error) {
	return &vaultv1.InfoResponse{Name: "mock", Version: "0.1.0", Platforms: []string{"darwin"}}, nil
}

// errorProvider always returns a gRPC status error from GetSecret.
type errorProvider struct {
	code codes.Code
	msg  string
}

func (e *errorProvider) GetSecret(_ context.Context, _ *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	return nil, status.Errorf(e.code, "%s", e.msg)
}

func (e *errorProvider) SetSecret(_ context.Context, _ *vaultv1.SetSecretRequest) (*vaultv1.SetSecretResponse, error) {
	return nil, status.Errorf(e.code, "%s", e.msg)
}

func (e *errorProvider) KeyExists(_ context.Context, _ *vaultv1.KeyExistsRequest) (*vaultv1.KeyExistsResponse, error) {
	return nil, status.Errorf(e.code, "%s", e.msg)
}

func (e *errorProvider) HealthCheck(
	_ context.Context,
	_ *vaultv1.HealthCheckRequest,
) (*vaultv1.HealthCheckResponse, error) {
	return &vaultv1.HealthCheckResponse{Available: true}, nil
}

func (e *errorProvider) Info(_ context.Context, _ *vaultv1.InfoRequest) (*vaultv1.InfoResponse, error) {
	return &vaultv1.InfoResponse{Name: "error", Version: "0.0.0"}, nil
}

func startServer(t *testing.T, p vault.Provider) *grpc.ClientConn {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	vaultv1.RegisterVaultProviderServiceServer(srv, vault.NewGRPCServer(p))
	go srv.Serve(lis) //nolint:errcheck
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestGRPCPlugin_GRPCServer(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcSrv := grpc.NewServer()
	plugin := vault.NewGRPCPlugin(&mockProvider{})
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

	client := vaultv1.NewVaultProviderServiceClient(conn)
	resp, err := client.GetSecret(context.Background(), &vaultv1.GetSecretRequest{Path: "test"})
	if err != nil {
		t.Fatalf("GetSecret() via registered server: %v", err)
	}
	if string(resp.Secret) != "test-secret" {
		t.Errorf("secret = %q, want %q", string(resp.Secret), "test-secret")
	}
}

func TestGRPCPlugin_GRPCClient(t *testing.T) {
	conn := startServer(t, &mockProvider{})

	plugin := vault.NewGRPCPlugin(&mockProvider{})
	raw, err := plugin.GRPCClient(context.Background(), nil, conn)
	if err != nil {
		t.Fatalf("GRPCClient() error: %v", err)
	}
	provider, ok := raw.(vault.Provider)
	if !ok {
		t.Fatal("GRPCClient() did not return a vault.Provider")
	}
	resp, err := provider.HealthCheck(context.Background(), &vaultv1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("HealthCheck() via GRPCClient provider: %v", err)
	}
	if !resp.Available {
		t.Error("Available = false, want true")
	}
}

func TestGRPCServer_GetSecret(t *testing.T) {
	server := vault.NewGRPCServer(&mockProvider{})
	resp, err := server.GetSecret(context.Background(), &vaultv1.GetSecretRequest{Path: "test"})
	if err != nil {
		t.Fatalf("GetSecret() error: %v", err)
	}
	if string(resp.Secret) != "test-secret" {
		t.Errorf("secret = %q, want %q", string(resp.Secret), "test-secret")
	}
}

func TestGRPCServer_HealthCheck(t *testing.T) {
	server := vault.NewGRPCServer(&mockProvider{})
	resp, err := server.HealthCheck(context.Background(), &vaultv1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("HealthCheck() error: %v", err)
	}
	if !resp.Available {
		t.Error("Available = false, want true")
	}
}

func TestGRPCServer_Info(t *testing.T) {
	server := vault.NewGRPCServer(&mockProvider{})
	resp, err := server.Info(context.Background(), &vaultv1.InfoRequest{})
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	if resp.Name != "mock" {
		t.Errorf("Name = %q, want %q", resp.Name, "mock")
	}
}

func TestNewClientConfig(t *testing.T) {
	cfg := vault.NewClientConfig("/usr/local/bin/locksmith-plugin-test")
	if cfg == nil {
		t.Fatal("NewClientConfig() returned nil")
	}
}

func TestGRPCClient_GetSecret_PreservesStatusCode(t *testing.T) {
	cases := []struct {
		name string
		code codes.Code
		msg  string
	}{
		{"NotFound", codes.NotFound, "keychain: item not found"},
		{"PermissionDenied", codes.PermissionDenied, "access denied"},
		{"Internal", codes.Internal, "unexpected vault error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn := startServer(t, &errorProvider{code: tc.code, msg: tc.msg})

			client := vault.NewGRPCClient(conn)
			_, gotErr := client.GetSecret(context.Background(), &vaultv1.GetSecretRequest{Path: "test"})
			if gotErr == nil {
				t.Fatal("GetSecret() returned nil error, want error")
			}

			var ve *sdkerrors.VaultError
			if !errors.As(gotErr, &ve) {
				t.Fatalf("error type = %T, want *sdkerrors.VaultError", gotErr)
			}
			if ve.Code != tc.code {
				t.Errorf("VaultError.Code = %v, want %v", ve.Code, tc.code)
			}
			if ve.Message != tc.msg {
				t.Errorf("VaultError.Message = %q, want %q", ve.Message, tc.msg)
			}
		})
	}
}

func TestGRPCClient_GetSecret(t *testing.T) {
	conn := startServer(t, &mockProvider{})
	client := vault.NewGRPCClient(conn)
	resp, err := client.GetSecret(context.Background(), &vaultv1.GetSecretRequest{Path: "test"})
	if err != nil {
		t.Fatalf("GetSecret() error: %v", err)
	}
	if string(resp.Secret) != "test-secret" {
		t.Errorf("secret = %q, want %q", string(resp.Secret), "test-secret")
	}
}

func TestGRPCClient_HealthCheck(t *testing.T) {
	conn := startServer(t, &mockProvider{})
	client := vault.NewGRPCClient(conn)
	resp, err := client.HealthCheck(context.Background(), &vaultv1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("HealthCheck() error: %v", err)
	}
	if !resp.Available {
		t.Error("Available = false, want true")
	}
}

func TestGRPCClient_Info(t *testing.T) {
	conn := startServer(t, &mockProvider{})
	client := vault.NewGRPCClient(conn)
	resp, err := client.Info(context.Background(), &vaultv1.InfoRequest{})
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	if resp.Name != "mock" {
		t.Errorf("Name = %q, want %q", resp.Name, "mock")
	}
}
