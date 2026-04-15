# Configuration Reference

Default config path: `~/.config/locksmith/config.yaml`

```yaml
defaults:
  session_ttl: 3h                            # default session duration
  socket_path: ~/.config/locksmith/locksmith.sock

logging:
  level: info                                # debug | info | warn | error
  format: text                               # text | json

vaults:
  keychain:
    type: keychain                           # macOS Keychain
  my-gopass:
    type: gopass
    store: personal                          # optional gopass store name

keys:
  github-token:
    vault: keychain
    path: "github-api-token"                 # account name in Keychain
  anthropic-key:
    vault: my-gopass
    path: "dev/anthropic"                    # path in gopass store
```

## Vault Types

| type | Description |
|------|-------------|
| `keychain` | macOS Keychain (CGo, Touch ID) |
| `gopass` | gopass password manager (shells out to `gopass` CLI) |
| `1password` | 1Password (shells out to `op` CLI) — future |
| `gnome-keyring` | GNOME Keyring (D-Bus) — future |

## Direct Access (Without Alias)

```bash
locksmith get --vault keychain --path my-account
locksmith get --vault my-gopass --path dev/key
```
