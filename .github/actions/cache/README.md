# CI cache policy

Repository-owned Actions caches are restored on all refs and saved only on `refs/heads/main`,
excluding `pull_request` and `pull_request_target` events. Missing dependencies on a PR are
fetched normally but are not uploaded as a PR-specific cache. Existing PR entries are not deleted
by this policy; the cache cleanup workflow handles retention separately.

Use this composite instead of `actions/cache` for explicit caches. It forwards the existing paths,
keys and restore prefixes unchanged and uses the upstream post-job save on eligible main runs.
Set `cache-save: false` when building custom source refs, even if the workflow itself runs on main.

## Setup actions

- `../setup-go`: retains native setup-go caching on eligible main runs and restores compatible
  caches elsewhere. `cache: false` disables both operations; `cache-save: false` disables only saves.
- `../setup-node-with-cache`: retains native setup-node pnpm caching on eligible main runs and
  restores compatible caches elsewhere. Install pnpm before calling it, as with setup-node.
- `../setup-java-and-gradle`: uses Gradle's `cache-read-only` input.
- `../setup-zig`: passes the caller's save opt-in to this composite.
- golangci-lint uses `skip-save-cache`; the checked-in CodeQL workflow uses `full` on main and
  `restore` elsewhere.

The Go and Node wrappers reproduce the pinned upstream restore keys and ordered cache paths.
When upgrading setup-go, setup-node or actions/cache, recheck key generation, runtime architecture,
Linux ImageOS, resolved Go version, dependency-file hashing, cache path order and cache-version
compatibility. The current wrappers target the repository's Linux/macOS x64/arm64 runners.

## GitHub-managed Code Quality

This policy covers checked-in workflows only. GitHub's separately managed Code Quality workflow
(`dynamic/github-code-quality/codeql`) is not configured by `.github/workflows/codeql.yml`.
Run 34518303358 for PR #3964 explicitly enabled dependency caching and saved a Go cache under
`refs/pull/3964/head`. Its caching requires a separate repository-settings/admin follow-up;
this PR must not be described as eliminating those managed cache writes.
