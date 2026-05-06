# Cutting a Release

This is the maintainer's checklist for shipping a new locksmith
version. It assumes the GPG signing setup described in
[CONTRIBUTING.md](../CONTRIBUTING.md#release-signing-setup) is
already in place.

## Procedure

1. **Clean main.** From a clean working tree on `main`:

   ```sh
   git status                # clean
   git pull --ff-only origin main
   ```

2. **Bump version + compress changelog.** Run the `version-bump`
   skill. It prompts for the new version, updates
   `sdk/version/VERSION`, and invokes the `changelog` skill to
   compress `## Development` into a `## Version vX.Y.Z` section in
   `CHANGES.md`. Verify the resulting section preserves all
   `BREAKING:` bullets at the top, in their original order
   (see [CONTRIBUTING.md "Changelog policy"](../CONTRIBUTING.md#changelog-policy-breaking-changes)).

3. **Optional: licenses.** The `version-bump` skill offers to invoke
   `check-licenses`. Accept if there have been dependency changes
   since the last release.

4. **Review.** `git diff` should show only `sdk/version/VERSION`,
   `CHANGES.md`, and (if licenses changed) `LICENSE`.

5. **Commit and tag.** Use a GPG-signed commit and an annotated tag:

   ```sh
   git add sdk/version/VERSION CHANGES.md LICENSE
   git commit -S -m "release: vX.Y.Z"
   git tag -s vX.Y.Z -m "vX.Y.Z"
   git push --follow-tags
   ```

6. **Watch CI.** The release workflow runs in three stages
   (validate -> build matrix + package -> publish):

   ```sh
   gh run watch
   ```

7. **Smoke-test.** After the workflow completes:

   ```sh
   docker run --rm -it ubuntu:24.04 sh -c '
     apt-get update && apt-get install -y curl unzip ca-certificates
     curl -fsSL https://github.com/lorem-dev/locksmith/releases/latest/download/install.sh | sh
     ~/.local/bin/locksmith version
   '
   ```

   The reported version must match the tag.

## Recovering from a failed release

If the `validate` job fails (e.g., `check-version` mismatch or a
flaky test), the release is not published. To retry:

```sh
# Fix locally, push fix commit.
git push origin main

# Delete the failed tag locally and remotely.
git tag -d vX.Y.Z
git push origin :refs/tags/vX.Y.Z

# Re-create the signed tag and push.
git tag -s vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

If `build` or `package` fails on a single matrix leg, re-running the
whole workflow (`gh run rerun <run-id>`) is usually enough. If a
specific runner is consistently red (e.g., a macOS image is broken),
file a follow-up to pin a known-good runner image.

## Release artifact inventory

A successful publish produces:

- `locksmith-linux-amd64.zip`
- `locksmith-linux-arm64.zip`
- `locksmith-darwin-amd64.zip`
- `locksmith-darwin-arm64.zip`
- `install.sh`
- `checksums.txt`
- `checksums.txt.asc`

Each zip contains a single file: `locksmith` (the bundle is embedded
inside the binary). The release notes body is the verbatim
`## Version vX.Y.Z` section of `CHANGES.md`.
