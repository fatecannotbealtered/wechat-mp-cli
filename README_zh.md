# wechat-mp-cli

[English](README.md) | [中文](README_zh.md)

[![CI](https://github.com/fatecannotbealtered/wechat-mp-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/fatecannotbealtered/wechat-mp-cli/actions/workflows/ci.yml)
[![npm version](https://img.shields.io/npm/v/@ananke/wechat-mp-cli.svg)](https://www.npmjs.com/package/@ananke/wechat-mp-cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

面向 AI Agent 的微信公众号 CLI。当前阶段走 API-first 路线：账号配置、token 检查、图片处理与上传、Markdown 创建草稿、草稿管理、发布生命周期、留言管理、图文统计、永久/临时素材、自定义菜单、远程 API 代理辅助和 webhook 验签。

## 为什么做

微信公众号后台偏浏览器操作，不适合 Agent 稳定调用。`wechat-mp-cli` 把常用发布流程收敛成可审计、可测试、可机器解析的 CLI 契约：

- 默认 JSON envelope 输出。
- 通过 `context`、`doctor`、`reference` 自描述能力。
- 写操作统一使用 `--dry-run` 到 `--confirm <confirm_token>`。
- AppSecret 本地加密保存，环境变量优先覆盖。
- 稳定 exit code 和 `E_*` 错误码，方便 Agent 判断重试、修参或请人介入。

最坏情况风险等级：**T2**。本工具可以用配置好的公众号凭据创建草稿，并提交公开发布任务。

## 安装

```bash
npm install -g @ananke/wechat-mp-cli
npx skills add fatecannotbealtered/wechat-mp-cli -y -g
```

本地开发：

```bash
make build
./bin/wechat-mp-cli context --compact
```

## 配置

配置文件：`~/.wechat-mp-cli/config.json`。

环境变量优先级最高：

| 变量 | 用途 |
| --- | --- |
| `WECHAT_MP_CLI_ACCOUNT` | 环境变量凭据对应的账号别名 |
| `WECHAT_MP_CLI_APP_ID` | 微信公众号 AppID |
| `WECHAT_MP_CLI_APP_SECRET` | 微信公众号 AppSecret |
| `WECHAT_MP_CLI_API_BASE` | API Base 覆盖，默认 `https://api.weixin.qq.com` |
| `WECHAT_MP_CLI_API_PROXY` | 可选 API 代理，例如 `socks5://127.0.0.1:1080` |

保存账号：

```bash
export WECHAT_SECRET=...
wechat-mp-cli setup account add --alias prod --app-id wx123 --secret-env WECHAT_SECRET --default --dry-run --compact
wechat-mp-cli setup account add --alias prod --app-id wx123 --secret-env WECHAT_SECRET --default --confirm <confirm_token> --compact
```

## 核心流程

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

所有写操作都必须使用 dry-run 返回的 `confirm_token` 再执行。确认令牌绑定 operation、payload hash、过期时间和本机 HMAC 密钥。

Markdown frontmatter 可以提供草稿元数据：

```yaml
---
title: 文章标题
author: Alice
summary: 简短摘要
cover: imgs/cover.png
sourceUrl: https://example.com/original
need_open_comment: 1
only_fans_can_comment: 0
---
```

确认执行后，本地正文图片会上传到微信正文图片接口，并把 `<img src>` 替换为微信返回的 URL。封面可以来自 `--cover-media-id`、`--cover-file`、frontmatter `cover`，也可以自动使用第一张本地正文图片。

## 远程 API 出口

如果本机 IP 不在微信公众号 API 白名单里，可以通过白名单服务器开 SSH SOCKS 隧道：

```bash
wechat-mp-cli remote ssh-command --host server.example.com --user deploy --local-port 1080 --compact
ssh -N -D 127.0.0.1:1080 deploy@server.example.com
wechat-mp-cli setup proxy set --url socks5://127.0.0.1:1080 --dry-run --compact
```

## 当前命令

| 领域 | 命令 |
| --- | --- |
| 自描述 | `context`, `doctor`, `reference`, `changelog`, `update --check` |
| 账号配置 | `setup account add/list/default/remove/test` |
| API 代理 | `setup proxy status/set/clear`, `remote ssh-command` |
| Token | `token status/refresh` |
| 渲染 | `render markdown/html` |
| 图片 | `image prepare/upload` |
| 素材 | `asset count/list/get/delete`, `asset temp upload/get/get-hd-voice` |
| 草稿 | `draft create/update/count/list/get/delete`, `draft switch status/enable` |
| 发布 | `publish submit/status/list/get-article/delete` |
| 留言 | `comment open/close/list/mark/unmark/delete/reply-add/reply-delete` |
| 数据 | `analytics article summary/total/read/read-hour/share/share-hour/published-read/published-share/published-summary/published-detail`, `analytics user summary/cumulate` |
| 菜单 | `menu get/set/delete/addconditional` |
| 二维码 | `qrcode create` |
| 粉丝 | `user info/list` |
| 粉丝标签 | `tag get/create/update/delete/members/tagging/untagging` |
| Webhook | `webhook verify` |

后续计划：更完整的微信排版主题和浏览器兜底。

## 开发

```bash
make fmt
make test
make build
npm install --package-lock-only --ignore-scripts
```

可运行示例放在 [examples/](examples/)，包括 frontmatter 文章和自定义菜单 JSON。

质量标准遵循 `ai-native-cli-spec`：README、Skill、`reference`、`context`、`doctor`、`changelog`、`update` 中声明的公开行为，应有命令级或包级测试保护。

## 链接

- Agent 入口：[AGENTS.md](AGENTS.md)
- Skill：[skills/wechat-mp-cli/SKILL.md](skills/wechat-mp-cli/SKILL.md)
- CLI 契约：[.agent/CLI-SPEC.md](.agent/CLI-SPEC.md)
- 官方端点覆盖说明：[docs/OFFICIAL_ENDPOINT_COVERAGE_zh.md](docs/OFFICIAL_ENDPOINT_COVERAGE_zh.md)
- 安全策略：[SECURITY.md](SECURITY.md)
- 变更记录：[CHANGELOG.md](CHANGELOG.md)
- 第三方声明：[NOTICE.md](NOTICE.md)
- 许可证：[MIT](LICENSE)
