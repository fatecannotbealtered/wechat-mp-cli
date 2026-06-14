# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
