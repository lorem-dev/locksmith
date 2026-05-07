# Installing Locksmith

The fastest path is the install script published with every release.
This document covers everything else: manual download, signature
verification, build-from-source, the `go install` fallback, and
uninstall.

## Quick install (recommended)

See the one-liner in [README.md](../README.md#installation).

The install script downloads a per-platform zip from the GitHub
release, verifies its SHA-256 against `checksums.txt`, extracts
`locksmith` to `~/.local/bin` (override with `LOCKSMITH_INSTALL_DIR`
or `--dir`), and refreshes bundled plugins by running
`locksmith plugins update --force`. Re-running the same command
updates an existing install in place.

## Install-script flags

| Flag | Env var | Default | Notes |
|------|---------|---------|-------|
| `--version vX.Y.Z` | `LOCKSMITH_VERSION` | `latest` | Pin a specific tag. |
| `--dir <path>` | `LOCKSMITH_INSTALL_DIR` | `~/.local/bin` | Target install directory. |
| `--no-plugins-update` | `LOCKSMITH_NO_PLUGINS_UPDATE=1` | unset | Skip the `plugins update --force` step. |
| `--via-go` | (none) | unset | Print `go install` instructions and exit non-zero (no automatic install). |
| `-h`, `--help` | (none) | - | Print usage and exit. |

Supported platforms: `linux/amd64`, `linux/arm64`, `darwin/arm64`.
On any other platform (including Intel Macs, darwin/amd64) the script
prints `go install` instructions and exits non-zero so the user can
opt in to a from-source build.

## Manual download

If you prefer to inspect everything yourself:

1. Pick a release at <https://github.com/lorem-dev/locksmith/releases>.
2. Download the zip for your platform: `locksmith-<os>-<arch>.zip`.
3. Download `checksums.txt` from the same release.
4. Verify:

   ```sh
   sha256sum -c checksums.txt --ignore-missing
   ```

   On macOS use `shasum -a 256 -c checksums.txt --ignore-missing`.
5. Unzip and move:

   ```sh
   unzip locksmith-darwin-arm64.zip
   chmod +x locksmith
   mv locksmith ~/.local/bin/
   ```
6. Optional: refresh plugins (`locksmith plugins update --force`).

## Verifying release integrity

Each release ships `checksums.txt` and a detached GPG signature
`checksums.txt.asc`. The install script verifies the SHA-256 checksum
automatically; the GPG step is opt-in and currently manual.

The full step-by-step verification flow, the public-key fingerprint,
and the rotation history live in
[`verification.md`](verification.md). Quick summary:

```sh
curl -fsSL https://raw.githubusercontent.com/lorem-dev/locksmith/main/.github/release-key.asc \
  | gpg --import
gpg --list-keys --fingerprint contact@lorem.dev      # compare to verification.md

TAG=v0.1.0
BASE="https://github.com/lorem-dev/locksmith/releases/download/${TAG}"
curl -fsSLO "${BASE}/checksums.txt"
curl -fsSLO "${BASE}/checksums.txt.asc"
gpg --verify checksums.txt.asc checksums.txt
sha256sum -c checksums.txt --ignore-missing          # shasum -a 256 -c on macOS
```

## `go install` fallback

If your platform is not in the supported list, the install script
prints step-by-step `go install` instructions and exits non-zero so
nothing happens automatically. You can then run the command yourself:

```sh
go install github.com/lorem-dev/locksmith/cmd/locksmith@latest
```

Caveats:

- The `go install` build does **not** contain real plugin or
  pinentry binaries. Its embedded bundle is a placeholder produced
  by `make ensure-bundle-placeholder`. `locksmith init` will warn
  that no plugins are bundled.
- Build plugins from source for your platform if you need them
  (`make build-all-plugins` in a clone of the repo, then drop the
  resulting `bin/locksmith-plugin-*` into `~/.config/locksmith/plugins/`).

## Build from source

```sh
git clone https://github.com/lorem-dev/locksmith
cd locksmith
make init
make build-all
sudo install -m 0755 bin/locksmith /usr/local/bin/
```

`make init` installs pinned tool versions (buf, protoc-gen-go,
golangci-lint) and generates protobuf code. `make build-all` builds
every plugin, packages them into the per-platform bundle, and
embeds the bundle into `bin/locksmith`.

## Updating

Re-run the install one-liner. The script detects the existing
binary, compares versions, and updates in place. It also re-runs
`locksmith plugins update --force`, which refreshes the extracted
plugin and pinentry binaries to match the new locksmith version.

## Uninstall

```sh
rm "${LOCKSMITH_INSTALL_DIR:-$HOME/.local/bin}/locksmith"
rm -rf ~/.config/locksmith
```

The first command removes the binary; the second removes
configuration, extracted plugins, and pinentry. Vault data
(passwords stored in gopass, macOS Keychain, etc.) is owned by the
underlying vault and is not touched.
