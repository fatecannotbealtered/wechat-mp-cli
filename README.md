# wechat-mp-cli

[English](README.md) | [中文](README_zh.md)

[![CI](https://github.com/fatecannotbealtered/wechat-mp-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/fatecannotbealtered/wechat-mp-cli/actions/workflows/ci.yml)
[![npm version](https://img.shields.io/npm/v/@fateforge/wechat-mp-cli.svg)](https://www.npmjs.com/package/@fateforge/wechat-mp-cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

AI-native CLI for WeChat Official Account operations. The current milestone is API-first: account setup, token checks, image processing/upload, Markdown-to-draft creation, draft management, publish lifecycle, comments, article analytics, permanent and temporary materials, custom menus, remote API proxy helpers, and webhook verification.

## Why This Exists

Most WeChat Official Account workflows are browser-heavy and hard for agents to operate safely. `wechat-mp-cli` exposes the workflow as a deterministic CLI contract:

- JSON envelope output by default.
- `context`, `doctor`, and `reference` for live self-description.
- `--dry-run` to `--confirm <confirm_token>` for writes.
- Local encrypted AppSecret storage with environment variable override.
- Stable exit codes and `E_*` error codes for agent recovery.

Worst-case risk tier: **T2**. This tool can create drafts and submit public publication jobs with the configured WeChat credential.

## Install

```bash
# Install the CLI (global npm).
npm install -g @fateforge/wechat-mp-cli
# Install the Agent Skill — copies into your agent-supported skills directory.
npx skills add fatecannotbealtered/wechat-mp-cli -y -g
```

Local development:

```bash
make build
./bin/wechat-mp-cli context --compact
```

## Configuration

Config file: `~/.wechat-mp-cli/config.json`.

Environment variables take precedence:

| Variable | Purpose |
| --- | --- |
| `WECHAT_MP_CLI_ACCOUNT` | Account alias for env-provided credentials |
| `WECHAT_MP_CLI_APP_ID` | WeChat Official Account AppID |
| `WECHAT_MP_CLI_APP_SECRET` | WeChat Official Account AppSecret |
| `WECHAT_MP_CLI_API_BASE` | API base URL override, defaults to `https://api.weixin.qq.com` |
| `WECHAT_MP_CLI_API_PROXY` | Optional API proxy, for example `socks5://127.0.0.1:1080` |

Add a saved account:

```bash
export WECHAT_SECRET=...
wechat-mp-cli setup account add --alias prod --app-id wx123 --secret-env WECHAT_SECRET --default --dry-run --compact
wechat-mp-cli setup account add --alias prod --app-id wx123 --secret-env WECHAT_SECRET --default --confirm <confirm_token> --compact
```

## Core Workflow

```bash
wechat-mp-cli context --compact
wechat-mp-cli doctor --compact
wechat-mp-cli reference --compact

wechat-mp-cli setup account test --account prod --compact
wechat-mp-cli token refresh --account prod --compact

wechat-mp-cli image upload cover.png --type material --account prod --dry-run --compact
wechat-mp-cli draft create --markdown article.md --account prod --dry-run --compact
wechat-mp-cli publish submit --media-id <draft_media_id> --account prod --dry-run --compact
wechat-mp-cli publish status --publish-id <publish_id> --account prod --compact
```

Writes must be repeated with the returned `confirm_token`. Tokens bind the operation, payload hash, expiry, and a machine-local HMAC secret.

Markdown frontmatter can supply draft metadata:

```yaml
---
title: Article title
author: Alice
summary: Short summary
cover: imgs/cover.png
sourceUrl: https://example.com/original
need_open_comment: 1
only_fans_can_comment: 0
---
```

Local inline images are uploaded to WeChat body-image storage and `<img src>` values are replaced with returned WeChat URLs after confirmation. The cover image can come from `--cover-media-id`, `--cover-file`, frontmatter `cover`, or the first local inline image.

## Remote API Egress

If your local IP is not in the WeChat API allowlist, run an SSH SOCKS tunnel through an allowlisted server:

```bash
wechat-mp-cli remote ssh-command --host server.example.com --user deploy --local-port 1080 --compact
ssh -N -D 127.0.0.1:1080 deploy@server.example.com
wechat-mp-cli setup proxy set --url socks5://127.0.0.1:1080 --dry-run --compact
```

## Current Commands

| Area | Commands |
| --- | --- |
| Self-description | `context`, `doctor`, `reference`, `changelog`, `update --check` |
| Account setup | `setup account add/list/default/remove/test` |
| API proxy | `setup proxy status/set/clear`, `remote ssh-command` |
| Token | `token status/refresh` |
| Rendering | `render markdown/html` |
| Images | `image prepare/upload` |
| Materials | `asset count/list/get/delete`, `asset temp upload/get/get-hd-voice` |
| Drafts | `draft create/update/count/list/get/delete`, `draft switch status/enable` |
| Publish | `publish submit/status/list/get-article/delete` |
| Comments | `comment open/close/list/mark/unmark/delete/reply-add/reply-delete` |
| Analytics | `analytics article summary/total/read/read-hour/share/share-hour/published-read/published-share/published-summary/published-detail`, `analytics user summary/cumulate` |
| Menu | `menu get/set/delete/addconditional` |
| QR codes | `qrcode create` |
| Followers | `user info/list` |
| Follower tags | `tag get/create/update/delete/members/tagging/untagging` |
| Webhook | `webhook verify` |

Planned next: richer WeChat typography themes and browser fallback.

## Development

```bash
make fmt
make test
make build
npm install --package-lock-only --ignore-scripts
```

Runnable examples live in [examples/](examples/), including a frontmatter article and a custom menu JSON payload.

The quality bar follows `ai-native-cli-spec`: public behavior documented in README, Skill, `reference`, `context`, `doctor`, `changelog`, and `update` should have command-level or package-level tests.

## Links

- Agent entry: [AGENTS.md](AGENTS.md)
- Skill: [skills/wechat-mp-cli/SKILL.md](skills/wechat-mp-cli/SKILL.md)
- CLI contract: [.agent/CLI-SPEC.md](.agent/CLI-SPEC.md)
- Official endpoint coverage: [docs/OFFICIAL_ENDPOINT_COVERAGE.md](docs/OFFICIAL_ENDPOINT_COVERAGE.md)
- Security: [SECURITY.md](SECURITY.md)
- Changelog: [CHANGELOG.md](CHANGELOG.md)
- Notice: [NOTICE.md](NOTICE.md)
- License: [MIT](LICENSE)
