// Internal (white-box) tests for plugin.Manager that need access to unexported types.
package plugin

import (
	"context"
	"errors"
	"io"
	"testing"

	goplugin "github.com/hashicorp/go-plugin"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
	"github.com/lorem-dev/locksmith/internal/log"
	"github.com/lorem-dev/locksmith/sdk/vault"
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

func (p *stubProvider) HealthCheck(
	_ context.Context,
	_ *vaultv1.HealthCheckRequest,
) (*vaultv1.HealthCheckResponse, error) {
	return nil, nil
}

func (p *stubProvider) Info(_ context.Context, _ *vaultv1.InfoRequest) (*vaultv1.InfoResponse, error) {
	return nil, nil
}

type infoFailingProvider struct{ stubProvider }

func (p *infoFailingProvider) Info(_ context.Context, _ *vaultv1.InfoRequest) (*vaultv1.InfoResponse, error) {
	return nil, errors.New("info rpc failed")
}

type infoStubProvider struct {
	stubProvider
	resp *vaultv1.InfoResponse
}

func (p *infoStubProvider) Info(_ context.Context, _ *vaultv1.InfoRequest) (*vaultv1.InfoResponse, error) {
	return p.resp, nil
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
	var provider vault.Provider = &stubProvider{}
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
	var provider vault.Provider = &stubProvider{}
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
	var provider vault.Provider = &stubProvider{}
	m.plugins["myvault"] = &runningPlugin{client: &stubClient{}, provider: provider}

	types := m.Types()
	if len(types) != 1 || types[0] != "myvault" {
		t.Errorf("Types() = %v, want [myvault]", types)
	}
}

func TestManager_Kill_WithPlugin(t *testing.T) {
	m := NewManager()
	sc := &stubClient{}
	var provider vault.Provider = &stubProvider{}
	m.plugins["myvault"] = &runningPlugin{client: sc, provider: provider}

	m.Kill()

	if !sc.killed {
		t.Error("Kill() should have called Kill on the client")
	}
	if len(m.plugins) != 0 {
		t.Errorf("Kill() should empty the plugins map, got %d entries", len(m.plugins))
	}
}

func TestKillOne(t *testing.T) {
	m := &Manager{
		plugins:       make(map[string]*runningPlugin),
		clientFactory: defaultClientFactory,
	}
	makeStub := func() *runningPlugin {
		return &runningPlugin{
			client: &stubClient{clientFn: func() (goplugin.ClientProtocol, error) {
				return nil, errors.New("not used")
			}},
			provider: nil,
		}
	}
	m.plugins["keychain"] = makeStub()
	m.plugins["gopass"] = makeStub()

	// Save client references before KillOne removes the entry from the map.
	gopassClient := m.plugins["gopass"].client.(*stubClient)
	keychainClient := m.plugins["keychain"].client.(*stubClient)

	m.KillOne("gopass")

	if _, exists := m.plugins["gopass"]; exists {
		t.Error("gopass should have been removed from plugins map")
	}
	if _, exists := m.plugins["keychain"]; !exists {
		t.Error("keychain should still be in plugins map")
	}
	if !gopassClient.killed {
		t.Error("KillOne should have called Kill() on the gopass client")
	}
	if keychainClient.killed {
		t.Error("KillOne should not have called Kill() on the keychain client")
	}
}

func TestKillOne_Unknown(t *testing.T) {
	m := NewManager()
	// Must not panic when called with an unknown vault type.
	m.KillOne("does-not-exist")
}

func TestLaunch_StoresWarnings_WhenInfoFails(t *testing.T) {
	m := NewManager()
	m.clientFactory = func(_ string) pluginClient {
		return &stubClient{
			clientFn: func() (goplugin.ClientProtocol, error) {
				return &stubProtocol{
					dispenseFn: func(_ string) (interface{}, error) {
						return &infoFailingProvider{}, nil
					},
				}, nil
			},
		}
	}
	if err := m.Launch("test", "/fake/binary"); err != nil {
		t.Fatalf("Launch() unexpected error: %v", err)
	}
	ws := m.Warnings("test")
	found := false
	for _, w := range ws {
		if w.Kind == WarnInfoUnavailable {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected WarnInfoUnavailable in %+v", ws)
	}
	if m.CachedInfo("test") != nil {
		t.Errorf("CachedInfo should be nil when Info() fails")
	}
}

func TestLaunch_CachesInfo_WhenInfoSucceeds(t *testing.T) {
	m := NewManager()
	wantInfo := &vaultv1.InfoResponse{
		Name:                "myvault",
		Version:             "0.1.0",
		Platforms:           []string{"darwin", "linux"},
		MinLocksmithVersion: "0.1.0",
		MaxLocksmithVersion: "0.1.0",
	}
	m.clientFactory = func(_ string) pluginClient {
		return &stubClient{
			clientFn: func() (goplugin.ClientProtocol, error) {
				return &stubProtocol{
					dispenseFn: func(_ string) (interface{}, error) {
						return &infoStubProvider{resp: wantInfo}, nil
					},
				}, nil
			},
		}
	}
	if err := m.Launch("myvault", "/fake/binary"); err != nil {
		t.Fatalf("Launch() unexpected error: %v", err)
	}
	got := m.CachedInfo("myvault")
	if got == nil || got.Name != "myvault" {
		t.Fatalf("CachedInfo = %+v, want name=myvault", got)
	}
}

func TestWarnings_UnknownVault(t *testing.T) {
	m := NewManager()
	if ws := m.Warnings("nope"); ws != nil {
		t.Errorf("Warnings(unknown) = %v, want nil", ws)
	}
}

func TestCachedInfo_UnknownVault(t *testing.T) {
	m := NewManager()
	if info := m.CachedInfo("nope"); info != nil {
		t.Errorf("CachedInfo(unknown) = %v, want nil", info)
	}
}
