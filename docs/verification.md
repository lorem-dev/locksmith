# Release verification

Locksmith releases ship with a SHA-256 checksum file and a detached GPG
signature over that file. Verifying both before installing ensures the
binary you run was produced by `.github/workflows/release.yml` and has
not been tampered with after the build.

## Public key

The release-signing public key is committed to this repository at
[`.github/release-key.asc`](../.github/release-key.asc) and is also
served raw via:

  https://raw.githubusercontent.com/lorem-dev/locksmith/main/.github/release-key.asc

| Field       | Value                                              |
|-------------|----------------------------------------------------|
| Owner       | `Lorem Dev Release <contact@lorem.dev>`            |
| Algorithm   | ed25519 (signing) + cv25519 (encryption subkey)    |
| Created     | 2026-05-07                                         |
| Expires     | 2031-05-06                                         |
| Fingerprint | `CFE6 485E 2351 9A25 A475  B900 AD0F 7A29 E439 8670` |

The fingerprint above is the canonical out-of-band reference. After
importing the key, compare what your `gpg` shows against this table.
If they differ, the key in the repository has been tampered with - do
NOT proceed with the install.

## Verifying a release

```sh
# 1. Import the public key
curl -fsSL https://raw.githubusercontent.com/lorem-dev/locksmith/main/.github/release-key.asc \
  | gpg --import

# 2. Confirm the fingerprint matches the table above
gpg --list-keys --fingerprint contact@lorem.dev

# 3. Download the checksum file and its detached signature
TAG=v0.1.0
BASE="https://github.com/lorem-dev/locksmith/releases/download/${TAG}"
curl -fsSLO "${BASE}/checksums.txt"
curl -fsSLO "${BASE}/checksums.txt.asc"

# 4. Verify the signature. A "Good signature" line confirms authenticity
gpg --verify checksums.txt.asc checksums.txt

# 5. Verify your downloaded artefact against the now-trusted checksum file
curl -fsSLO "${BASE}/locksmith-linux-amd64.zip"
sha256sum -c checksums.txt --ignore-missing
# macOS users: shasum -a 256 -c checksums.txt --ignore-missing
```

A successful run prints
`Good signature from "Lorem Dev Release <contact@lorem.dev>"` followed
by `locksmith-<platform>.zip: OK`. Any other outcome means the file
should be discarded and re-downloaded.

> The install script (`install.sh`) verifies the SHA-256 checksum
> automatically but does NOT verify the GPG signature - the GPG step is
> opt-in and currently manual. Pair `install.sh` with the steps above
> for full end-to-end verification.

## Key rotation

The release key is rotated every five years, or sooner on suspected
compromise. Older keys remain in the repository so historical releases
stay verifiable.

| Generated  | Fingerprint                                          | Expires    | Used for       |
|------------|------------------------------------------------------|------------|----------------|
| 2026-05-07 | `CFE6 485E 2351 9A25 A475  B900 AD0F 7A29 E439 8670` | 2031-05-06 | v0.1.0 onwards |

## See also

- [`install.md`](install.md) - install paths and the install-script
  flags
- [`../CONTRIBUTING.md#release-signing-setup`](../CONTRIBUTING.md#release-signing-setup) -
  how the maintainer generates and rotates the key
