# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.4] - 2026-06-16

### Changed

- `update` is rewritten from package-manager delegation (npm/go install) to a self-contained verified binary self-update: download the release archive + `checksums.txt` + Sigstore bundle, verify the signature **in-process** (embedded `sigstore-go`, embedded TUF root) against this repo's tagged release-workflow identity, verify the archive SHA256, and replace the running binary — no dependency on npm/go/pip being installed. Releases are signed with `cosign sign-blob --new-bundle-format`.

### Security

- Verification is mandatory and fail-closed (no skip path); release-integrity failures return the non-retryable `E_INTEGRITY` code (exit 1) instead of a retryable network code.

## [1.0.3] - 2026-06-15

### Added

- Batch operations (CLI-SPEC §15): `message mass sendall`/`send`/`preview`/`get`/`delete` broadcast a message to all followers, a tag, or an explicit openid list (≤10000, one async job) — all gated by `--dry-run` → `--confirm` plus `--dangerous` for the critical/high writes; poll delivery with `message mass get --msg-id`. `user info-batch --openids` fetches up to 100 follower profiles per call, auto-chunking longer lists, and returns one aggregated `items[]` (keyed by openid `target`) plus a `summary{total,succeeded,failed}` so a per-item failure never fails the whole command (`--continue-on-error`, default `true`).

### Changed

- npm scope 迁移 `@fatecannotbealtered-` → `@fateforge`（无横线 org 在 npm 被占，迁移到 `@fateforge`）。主包与各平台包（`@fateforge/wechat-mp-cli-<os>-<arch>`）一并改名；GitHub org / go module path（`github.com/fatecannotbealtered/...`）与 release 源保持不变。

### Fixed

- `message mass sendall`: enforce `--to-all` and `--tag-id` as mutually exclusive. Previously giving both let `--to-all` silently win; now the audience guard requires exactly one and returns `E_VALIDATION` when both are set.

## [1.0.2] - 2026-06-14

### Added

- `qrcode create` (ticket + showqrcode URL); `user info` / `user list`; `tag` CRUD + `tag members` + batch `tag tagging`/`untagging`; `menu addconditional` (personalized menus).
- `reference` now exposes a real per-command `output_schema` + `examples[]`, guarded against regression.

### Changed

- Confirm tokens are now single-use (E_CONFLICT on replay) for write commands.

## [1.0.1] - 2026-06-14

### Added

- Expanded WeChat `errcode` mapping so agents get an accurate `error.code` + `retryable`: `40164` (IP not in allowlist) → `E_FORBIDDEN` with an actionable hint pointing at `remote ssh-command` / `setup proxy set` (the recovery path the proxy feature exists for); `42001` (access_token expired) → `E_AUTH` retryable; `40007`/`41030` (bad media_id / invalid menu) → `E_NOT_FOUND`.

### Fixed

- `changelog` now reads CHANGELOG.md embedded into the binary (`go:embed`), so it returns real content regardless of the working directory the agent runs the binary from; previously it read `./CHANGELOG.md` from the CWD and fell back to a stub everywhere else.

## [1.0.0] - 2026-06-14

First stable release: recorded live smoke against the real WeChat Official Account API (`docs/LIVE-SMOKE-EVIDENCE.md`); `release_readiness` is `stable` with `live_smoke_status: verified`.

### Added

- Recorded live smoke against the real WeChat API (2026-06-14): `stable_token` lifecycle, asset/draft/menu reads, the `--dangerous` T2 write gate, and a real temporary-material upload through the dry-run/confirm chain.
- Initial Go implementation for the AI-native WeChat Official Account CLI.
- Self-description commands: `context`, `doctor`, `reference`, `changelog`, and `update --check`.
- Local account configuration with encrypted AppSecret storage and environment variable overrides.
- HMAC-backed `--dry-run` to `--confirm <confirm_token>` flow for write commands.
- WeChat API client for token refresh, image upload, draft add/batchget/get/delete, publish submit, and publish status.
- Goldmark GFM Markdown renderer with frontmatter metadata support.
- Local image inspection, conversion, and compression before WeChat uploads.
- Automatic local inline image upload and `<img src>` replacement during draft creation.
- Draft update and draft count commands.
- Published article lifecycle commands: list, get article, and delete.
- Comment management commands: open, close, list, mark/unmark featured, delete, reply add, and reply delete.
- Article and user analytics commands backed by WeChat datacube endpoints.
- Published-content analytics commands for WeChat's `getarticleread`, `getarticleshare`, `getbizsummary`, and `getarticletotaldetail` endpoints.
- API proxy support through `WECHAT_MP_CLI_API_PROXY` and `setup proxy`.
- `remote ssh-command` helper for SSH SOCKS egress through an allowlisted server.
- Permanent material commands: `asset count/list/get/delete`.
- Temporary media commands: `asset temp upload/get/get-hd-voice`.
- Draft/publish switch commands: `draft switch status/enable`.
- Custom menu commands: `menu get/set/delete`.
- Webhook verification command: `webhook verify`.
- Official endpoint coverage documentation under `docs/`.
- MVP command groups: `setup account`, `setup proxy`, `remote`, `token`, `render`, `image`, `asset`, `draft`, `publish`, `menu`, and `webhook`.
- Package-level tests for confirmation tokens, config loading, rendering, API client behavior, and reference metadata.

### Changed

### Fixed

### Deprecated

### Removed

### Security

- Write commands require operation-bound confirmation tokens.
- Token refresh output redacts access token values by default.
- Public publication, material deletion, menu changes, credential changes, and proxy changes are modeled as confirmed writes.

<!--
Copy the block below for each release. Newest version first.
Keep the link references at the bottom of the file in sync.

## [0.1.0] - 2026-06-12

### Added

- Access tokens now come from `/cgi-bin/stable_token` (does not invalidate tokens held by other services on the same AppID) and are cached encrypted in `~/.wechat-mp-cli/token-cache.json`, reused until a 5-minute expiry margin; `token status` reads the cache without minting, `token refresh` forces a new token, and `context`/`doctor` report token validity and expiry (CLI-SPEC §15.1).
- App secrets now live in the OS keyring per app_id (SEC-SPEC §4 keyring three-part pattern); `config.json` keeps zero secrets with a `secret_storage` marker, machine-bound AES file encryption remains as the visible fallback, and account removal clears the keyring entry and cached token.
- T2 second gate: high/critical risk writes (publish submit/delete, draft create/update/delete, menu set/delete, asset delete, comment open/close/delete/reply-add, draft switch enable) require `--dangerous` in both the dry-run and confirm steps (SEC-SPEC §3).
- External WeChat content (comment threads, draft and published article bodies, asset items) is tagged `_untrusted` (SEC-SPEC §2).
- Redacted audit logging for write commands under `~/.wechat-mp-cli/audit/` (monthly JSONL, 3-month retention, `--quiet` cannot disable it; opt out with `WECHAT_MP_CLI_NO_AUDIT=1`).
- `reference` exposes `release_readiness` (beta: FCC verified via the enumeration guard, live smoke evidence missing) and `doctor` reports the matching check plus a `credentials` lifecycle check.
- FCC enumeration guard (`TestFCC_EveryLeafCommandHasTest`) and a boundary test suite (`test/e2e`) driving all 64 leaf commands through the real binary against a mock WeChat upstream, including the dry-run→confirm cycle, dangerous-gate, `_untrusted`, and token-cache contract tests.
- First public release.

### Changed

### Fixed

### Deprecated

### Removed

### Security

[Unreleased]: https://github.com/fatecannotbealtered/wechat-mp-cli/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/fatecannotbealtered/wechat-mp-cli/releases/tag/v0.1.0
-->
