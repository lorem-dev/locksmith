# Contributing to Locksmith

## License Compliance

All direct third-party dependencies must carry a license compatible with
Apache 2.0 before being merged.

**Not acceptable** (prohibit commercial use or impose copyleft
incompatible with Apache 2.0):

- GPL-2.0 / GPL-3.0 / AGPL-3.0
- LGPL-2.1 (Go links statically, so the dynamic-linking exception
  does not apply)
- SSPL-1.0 / BSL-1.1
- Any Creative Commons -NC- variant
- Any license carrying a "Commons Clause" addendum

If the library you need carries one of these, look for a permissive
alternative or raise the issue with the maintainers.

When adding or removing a library, run the `check-licenses` skill
after editing `go.mod`. The skill updates the `LICENSE` "Third-Party
Notices" section automatically.

## Branching model

Locksmith uses two long-lived branches:

- **`develop`** is the integration branch. New features, refactors,
  and non-urgent fixes land here either via PRs from short-lived
  feature branches or directly by maintainers. `## Development` in
  `CHANGES.md` tracks everything that has merged since the last
  release.
- **`main`** holds released versions only. Every commit on `main` is
  either a release merge from `develop` or - rarely - a hotfix.

**Direct commits to `main` are not allowed** outside of hotfixes.
When opening a PR, the base branch is `develop` unless the change is
explicitly a release or hotfix.

### Release flow

1. Work accumulates on `develop`; each user-visible change gets a
   bullet under `## Development` in `CHANGES.md` in the same commit.
2. When ready to release, on `develop` run the `release-prep` skill.
   It verifies that each accumulated bullet has matching docs
   updates since the last tag, then invokes `version-bump` (which
   bumps `sdk/version/VERSION`, syncs the README install pin, and
   invokes `changelog` to compress `## Development` into a new
   `## Version vX.Y.Z` section).
3. Commit the bump + changelog as `release: vX.Y.Z` (GPG-signed) and
   push `develop`.
4. Open a PR from `develop` into `main` with the new
   `## Version vX.Y.Z` body as the description.
5. Merge the PR. **The merge commit on `main` is the release
   commit.**
6. Tag the merge commit on `main`:

   ```bash
   git checkout main && git pull
   git tag -s vX.Y.Z -m "vX.Y.Z"
   git push --tags
   ```

7. The tag triggers the release workflow, which builds binaries,
   signs `checksums.txt`, and publishes a GitHub release using the
   `## Version vX.Y.Z` body as the release notes.

### Hotfix flow

For an urgent fix to an already-released version:

1. Branch from `main` (e.g. `hotfix/cve-2026-xxxx`).
2. Apply the fix and a `CHANGES.md` entry. Bump the patch number on
   `sdk/version/VERSION`.
3. Open a PR into `main`. After merging, tag the merge commit as
   above.
4. Sync `develop`:
   - Cherry-pick or merge the fix commit into `develop` so the next
     release does not regress it.
   - **Remove the hotfix bullet from `develop`'s `## Development`
     section** - it has already shipped on `main` as `vX.Y.(Z+1)`
     and must not be counted again when the next minor/major
     release is cut.
   - Update `develop`'s `sdk/version/VERSION` if the next release on
     `develop` should now bump from `vX.Y.(Z+1)` rather than the
     pre-hotfix base.

## Versioning

Locksmith follows [Semantic Versioning](https://semver.org/):
`MAJOR.MINOR.PATCH`. The canonical version lives in
`sdk/version/VERSION`; the repo-root `VERSION` symlink points to it.
The file content has no `v` prefix; git tags do (e.g. `v0.1.0`).
`sdk/version.Current` is populated at build time via `//go:embed`.

`make check-version` asserts that the git tag matches `VERSION` and
that `CHANGES.md` has a `## Version vX.Y.Z` section for the tagged
version. It reads the tag from `$GITHUB_REF` (GitHub Actions) or
`$CI_COMMIT_TAG` (GitLab CI); on non-tag builds it is a no-op. CI
runs it on every tag build:

```yaml
# GitHub Actions
- name: Verify release version
  if: startsWith(github.ref, 'refs/tags/v')
  run: make check-version
```

```yaml
# GitLab CI
verify-version:
  rules: [{ if: '$CI_COMMIT_TAG =~ /^v/' }]
  script: [make check-version]
```

On non-tag builds the target is a no-op.

## GPG Signing

Signing commits is strongly recommended. Maintainers verify that
commits come from you and have not been tampered with.

```bash
git commit -S -m "feat: ..."
git config commit.gpgsign true   # default for this repo
```

Re-sign unsigned commits before opening a PR:

```bash
LAST_SIGNED=$(git log --format="%G? %H" | awk '$1=="G"{print $2; exit}')
git rebase "$LAST_SIGNED" --exec "git commit --amend --no-edit -S"
```

## Release signing (CI)

The release workflow signs `checksums.txt` with a dedicated GPG key
that is separate from any maintainer's personal commit-signing key.
The signing itself happens in CI; the steps below are the one-time
bootstrap (or 5-year rotation) of the key that CI uses.

1. Generate a dedicated release key:

   ```sh
   gpg --batch --gen-key <<EOF
   %no-protection
   Key-Type: EDDSA
   Key-Curve: ed25519
   Subkey-Type: ECDH
   Subkey-Curve: cv25519
   Name-Real: Lorem Dev Release
   Name-Email: contact@lorem.dev
   Expire-Date: 5y
   EOF
   ```

   Drop the `%no-protection` line to add a passphrase; you will
   then have to supply `LOCKSMITH_RELEASE_GPG_PASSPHRASE` in step 4.

2. Export both halves:

   ```sh
   FPR="$(gpg --list-secret-keys --with-colons contact@lorem.dev \
        | awk -F: '/^fpr:/ {print $10; exit}')"
   gpg --armor --export-secret-keys "$FPR" > release-key.priv.asc
   gpg --armor --export             "$FPR" > release-key.asc
   ```

3. Commit the public key (GPG-signed):

   ```sh
   mkdir -p .github
   mv release-key.asc .github/release-key.asc
   git add .github/release-key.asc
   git commit -S -m "chore(release): add release-signing public key"
   ```

   Record the fingerprint and expiry in
   [`docs/verification.md`](docs/verification.md). The current key
   is fingerprint
   `CFE6 485E 2351 9A25 A475  B900 AD0F 7A29 E439 8670`, generated
   2026-05-07, expires 2031-05-06.

4. Add the GitHub Actions secrets in
   `Settings -> Secrets and variables -> Actions`:

   - `LOCKSMITH_RELEASE_GPG_KEY` = contents of `release-key.priv.asc`.
   - `LOCKSMITH_RELEASE_GPG_PASSPHRASE` = the passphrase (or empty).

5. Securely destroy the local secret material:

   ```sh
   shred -u release-key.priv.asc
   gpg --delete-secret-keys "$FPR"   # optional
   ```

6. **Rotation.** Every five years (or on suspected compromise),
   repeat steps 1-4. Do NOT delete prior public keys from the repo;
   users may still verify older releases with them.

7. **Verify the next release.** After the first tagged release with
   this key, verify locally:

   ```sh
   curl -fsSLO https://github.com/lorem-dev/locksmith/releases/latest/download/checksums.txt
   curl -fsSLO https://github.com/lorem-dev/locksmith/releases/latest/download/checksums.txt.asc
   gpg --verify checksums.txt.asc checksums.txt
   ```

   A `Good signature` line confirms CI is signing with the intended
   key.

## Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

Relates #<issue>
```

**Types:** `feat`, `fix`, `chore`, `docs`, `test`, `refactor`,
`perf`, `ci`, `build`.

**Rules:**

- English, imperative mood ("add", not "added").
- Subject line under 72 chars.
- Reference issues with `Relates #123` in the footer when applicable.
- Use `(scope)` only when it already exists in the git log; prefer
  no scope over inventing one. Check with
  `git log --oneline | grep "type(" | head -20`.
- No mentions of AI tools, agents, or assistants anywhere - no
  `Co-Authored-By: Claude ...`, no "generated by", no tool names.

**Examples:**

```
feat(session): add TTL-based expiry with memory wipe
fix(keychain): handle errSecUserCanceled from Touch ID prompt
chore: update golangci-lint to v1.57
```

## Test Coverage and Style

- Minimum coverage per package: **90%** (`make test-coverage`).
- Race detector must pass (`make test-race`).
- Integration tests: `make test-integration`.
- Follow `golangci-lint` rules in `.golangci.yml`.
- All exported symbols must have godoc comments.
- CLI command files in `internal/cli/` must be named
  `<command>_cmd.go` (e.g. `get_cmd.go`); tests go in
  `<command>_cmd_test.go`. Non-command files (`root.go`,
  `client.go`, `color.go`, `errors.go`) are exempt.

`make verify` bundles lint, race, coverage, build, GPG signatures,
docs completeness, and CHANGES.md checks - run it before submitting.

## Plugin versioning and compatibility

Bundled plugins (`plugins/gopass`, `plugins/keychain`) and any
third-party plugins follow the rules below. `make test-race`
enforces them - a plugin PR cannot land green without satisfying
every check.

### Bundled plugins (lockstep with host)

A bundled plugin has no separate version field, tag, or release
cycle. Its version is the version of the `locksmith` binary that
bundled it (see [PLUGINS.md](PLUGINS.md)).

Each bundled plugin's `Info()` MUST set:

- `MaxLocksmithVersion = sdkversion.Current` (using
  `github.com/lorem-dev/locksmith/sdk/version`). Enforced by unit
  tests in `plugins/<name>/provider_test.go`.
- `MinLocksmithVersion` - the earliest host version the plugin is
  still tested against. Bumped only when the plugin starts using an
  SDK feature unavailable in earlier hosts; record the bump in
  `CHANGES.md` under `## Development`.
- `Platforms` - list of `runtime.GOOS` values, drawn from
  `sdk/platform`. Empty list means "skip the platform check" and
  SHOULD NOT be used by bundled plugins.

### Third-party plugins

Authors own their own versioning and tag cadence.

- `MinLocksmithVersion` is strongly recommended; omitting it
  produces a `min_version_missing` warning visible in
  `locksmith vault health`.
- `MaxLocksmithVersion` is optional but recommended. Format
  `major.minor.patch`, no `v` prefix, no pre-release suffix
  (validated by `internal/plugin.CompatValidator`).
- Use `sdk/platform` constants for `Platforms`.
- Add unit tests covering `Info()`, modelled on
  `plugins/gopass/provider_test.go`.

### SDK / proto breaking changes

Any non-additive change to `sdk/vault.Provider`, `sdk/errors`,
`sdk/log`, `sdk/session`, `sdk/platform`, or `proto/` is a breaking
change for plugin authors. The same commit MUST:

1. Update all bundled plugins; the green race-test job confirms the
   workspace compiles.
2. Document the migration in `docs/plugins/authoring.md` or
   `docs/plugins/compatibility.md`.
3. Add a `CHANGES.md ## Development` entry beginning with
   `BREAKING:` (see "Changelog policy" below).
4. Bump bundled plugins' `MinLocksmithVersion` to the upcoming
   release tag, signalling the floor for third-party plugins.

### Pointers

- Plugin SDK quickstart: [docs/plugins/authoring.md](docs/plugins/authoring.md).
- Wire-level compatibility: [docs/plugins/compatibility.md](docs/plugins/compatibility.md).
- Bundle pipeline: [docs/plugins/architecture.md](docs/plugins/architecture.md).
- Plugin overview for end users: [PLUGINS.md](PLUGINS.md).

## Changelog policy

Every user-visible change must have a bullet under `## Development`
in `CHANGES.md`, written in the same commit. See [CLAUDE.md](CLAUDE.md)
for the project-wide rule.

### What is a breaking change?

Any of:

- Alters the daemon-plugin protocol (`proto/`, `sdk/vault.Provider`,
  `sdk/errors`, `sdk/log`, `sdk/session`, `sdk/platform`).
- Removes or renames a CLI flag, command, or environment variable.
- Changes `config.yaml` schema in a non-additive way.
- Drops support for a previously documented platform or host
  version.

Breaking changes MUST be authored as a bullet beginning with
`BREAKING:` (uppercase, colon, then space), in the same commit as
the change. The `changelog` skill is the source of truth for how
`BREAKING:` bullets are grouped, preserved across compressions, and
treated as immutable after release - consult its `SKILL.md` for the
detailed authoring rules.

### Release-time guarantee

The release workflow's `extract-changelog` step reads the
`## Version vX.Y.Z` section matching the pushed tag and uses its
full body (with all `BREAKING:` bullets grouped at the top) as the
GitHub-release body. The published changelog IS the release notes;
there is no separate release-notes file.

The workflow refuses to publish if the section is missing or empty.
