// Internal (white-box) tests for plugin.Manager that need access to unexported types.
package plugin

import (
	"context"
	"errors"
	"io"
	"testing"

	goplugin "github.com/hashicorp/go-plugin"
	sdk "github.com/lorem-dev/locksmith/sdk"
	"github.com/lorem-dev/locksmith/internal/log"
	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
)

func init() {
	log.Init(io.Discard, "error", "text")
}

// --- mock helpers ---

type stubClient struct {
	killed   bool
	clientFn func() (goplugin.ClientProtocol, error)
}

func (sc *stubClient) Client() (goplugin.ClientProtocol, error) {
	if sc.clientFn != nil {
		return sc.clientFn()
	}
	return nil, errors.New("stub: no clientFn")
}

func (sc *stubClient) Kill() { sc.killed = true }

type stubProtocol struct {
	dispenseFn func(string) (interface{}, error)
}

func (sp *stubProtocol) Dispense(name string) (interface{}, error) {
	if sp.dispenseFn != nil {
		return sp.dispenseFn(name)
	}
	return nil, errors.New("stub: no dispenseFn")
}

func (sp *stubProtocol) Close() error { return nil }
func (sp *stubProtocol) Ping() error  { return nil }

type stubProvider struct{}

func (p *stubProvider) GetSecret(_ context.Context, _ *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	return nil, nil
}
func (p *stubProvider) HealthCheck(_ context.Context, _ *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error) {
	return nil, nil
}
func (p *stubProvider) Info(_ context.Context, _ *vaultv1.InfoRequest) (*vaultv1.InfoResponse, error) {
	return nil, nil
}

// --- tests ---

func TestLaunch_ClientConnectError(t *testing.T) {
	m := NewManager()
	m.clientFactory = func(_ string) pluginClient {
		return &stubClient{
			clientFn: func() (goplugin.ClientProtocol, error) {
				return nil, errors.New("connection refused")
			},
		}
	}
	err := m.Launch("test", "/fake/binary")
	if err == nil {
		t.Fatal("Launch() expected error when client fails to connect")
	}
}

func TestLaunch_DispenseError(t *testing.T) {
	m := NewManager()
	m.clientFactory = func(_ string) pluginClient {
		return &stubClient{
			clientFn: func() (goplugin.ClientProtocol, error) {
				return &stubProtocol{
					dispenseFn: func(_ string) (interface{}, error) {
						return nil, errors.New("dispense error")
					},
				}, nil
			},
		}
	}
	err := m.Launch("test", "/fake/binary")
	if err == nil {
		t.Fatal("Launch() expected error when dispense fails")
	}
}

func TestLaunch_NotProvider(t *testing.T) {
	m := NewManager()
	m.clientFactory = func(_ string) pluginClient {
		return &stubClient{
			clientFn: func() (goplugin.ClientProtocol, error) {
				return &stubProtocol{
					dispenseFn: func(_ string) (interface{}, error) {
						return "not a provider", nil
					},
				}, nil
			},
		}
	}
	err := m.Launch("test", "/fake/binary")
	if err == nil {
		t.Fatal("Launch() expected error when raw is not a Provider")
	}
}

func TestLaunch_Success(t *testing.T) {
	m := NewManager()
	var provider sdk.Provider = &stubProvider{}
	m.clientFactory = func(_ string) pluginClient {
		return &stubClient{
			clientFn: func() (goplugin.ClientProtocol, error) {
				return &stubProtocol{
					dispenseFn: func(_ string) (interface{}, error) {
						return provider, nil
					},
				}, nil
			},
		}
	}
	if err := m.Launch("test", "/fake/binary"); err != nil {
		t.Fatalf("Launch() unexpected error: %v", err)
	}
	got, err := m.Get("test")
	if err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	if got != provider {
		t.Error("Get() returned wrong provider")
	}
}

func TestLaunch_AlreadyRunning(t *testing.T) {
	m := NewManager()
	var provider sdk.Provider = &stubProvider{}
	m.clientFactory = func(_ string) pluginClient {
		return &stubClient{
			clientFn: func() (goplugin.ClientProtocol, error) {
				return &stubProtocol{
					dispenseFn: func(_ string) (interface{}, error) {
						return provider, nil
					},
				}, nil
			},
		}
	}
	if err := m.Launch("test", "/fake/binary"); err != nil {
		t.Fatalf("first Launch() unexpected error: %v", err)
	}
	err := m.Launch("test", "/fake/binary")
	if err == nil {
		t.Fatal("second Launch() expected error for already-running plugin")
	}
}

func TestManager_Types_WithPlugin(t *testing.T) {
	m := NewManager()
	var provider sdk.Provider = &stubProvider{}
	m.plugins["myvault"] = &runningPlugin{client: &stubClient{}, provider: provider}

	types := m.Types()
	if len(types) != 1 || types[0] != "myvault" {
		t.Errorf("Types() = %v, want [myvault]", types)
	}
}

func TestManager_Kill_WithPlugin(t *testing.T) {
	m := NewManager()
	sc := &stubClient{}
	var provider sdk.Provider = &stubProvider{}
	m.plugins["myvault"] = &runningPlugin{client: sc, provider: provider}

	m.Kill()

	if !sc.killed {
		t.Error("Kill() should have called Kill on the client")
	}
	if len(m.plugins) != 0 {
		t.Errorf("Kill() should empty the plugins map, got %d entries", len(m.plugins))
	}
}
