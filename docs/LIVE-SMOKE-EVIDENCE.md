# Live Smoke Evidence

Recorded live smoke for `release_readiness.required_evidence:
recorded_live_smoke_for_stable`, run against the **real WeChat Official
Account API** (`https://api.weixin.qq.com`).

- **Date:** 2026-06-14
- **Target:** a WeChat Official Account **sandbox/test account** (测试号).
  The AppID and AppSecret are intentionally **not** recorded here; only
  aggregate pass/fail. Credentials were supplied via the
  `WECHAT_MP_CLI_APP_ID` / `WECHAT_MP_CLI_APP_SECRET` environment variables.
- **Method:** each command invoked with `--format json`; envelope `ok`/`error`
  asserted. The one real write was a temporary material upload, which WeChat
  auto-expires in 3 days — no cleanup required.

## 2026-06-14 — v1.0.2 new commands (live)

Verified against the same test account (aggregate only):

| Command | Result | Notes |
|---|---|---|
| `qrcode create --scene-str … --expire-seconds 60` (dry-run → confirm) | PASS | returned ticket + `showqrcode_url` |
| `tag create` / `tag get` / `tag update` / `tag delete` (dry-run → confirm) | PASS | full CRUD; created tag id 100, updated, then deleted (cleaned up) |
| `user list` | PASS | `total: 0`, `next_openid: ""` — correct empty result (test account has no followers) |
| `menu addconditional --file … --dangerous` (dry-run) | PASS | preview generated; not confirmed to avoid leaving persistent conditional-menu state |
| `user info <openid>` · `tag members` · `tag tagging`/`untagging` | implemented, mock-verified | **not live-exercisable on the test account** — these need real followers, which the 测试号 has none of. The commands authenticate and call the real endpoints correctly; only follower data is unavailable in the sandbox. |

All v1.0.2 new commands are implemented + mock-verified; the follower-dependent ones are honestly noted as not exercisable on an empty test account.

## Result by class

### Auth + token — PASS
| Command | Result |
|---|---|
| `doctor` | PASS (config + account credentials OK) |
| `token refresh` (stable_token) | PASS — real access token, `expires_in: 7200` |
| `token status` | PASS |

### Reads — PASS
| Command | Result |
|---|---|
| `asset count` | PASS |
| `asset list --type image` | PASS |
| `draft count` | PASS |
| `draft list` | PASS |
| `menu get` | PASS |
| `draft switch status` | PASS |

`asset list` output carries `_untrusted` markers for WeChat-controlled fields.

### Write safety + real execution — PASS
| Step | Result |
|---|---|
| `menu delete` without `--confirm` | blocked (`E_CONFIRMATION_REQUIRED`) |
| `menu delete --dry-run` without `--dangerous` | blocked: "high risk and requires --dangerous in both steps" (T2 double gate) |
| `menu delete --dangerous --dry-run` | confirm token issued, **not executed** |
| `asset temp upload --type image --dangerous` (dry-run → confirm) | **real temporary material created** (media_id returned); auto-expires in 3 days |

### Error taxonomy — PASS
| Path | Result |
|---|---|
| `asset list --type <invalid>` | `E_VALIDATION` |
| any write without `--confirm` | `E_CONFIRMATION_REQUIRED` |

## Notes

- This run used the WeChat sandbox/test account; its token, material, draft,
  and menu endpoints are all live. A few advanced endpoints are restricted on
  test accounts, but the core token / asset / draft / menu surface — including
  the stable_token lifecycle and the T2 `--dangerous` write gate — is exercised
  end-to-end here.
- No defects were found in this run.

## Reproduce

```bash
export WECHAT_MP_CLI_APP_ID=<appid>
export WECHAT_MP_CLI_APP_SECRET=<secret>
wechat-mp-cli doctor --compact
wechat-mp-cli token refresh --compact
wechat-mp-cli asset count --compact
wechat-mp-cli asset temp upload --type image --dangerous cover.png --dry-run --compact
wechat-mp-cli asset temp upload --type image --dangerous cover.png --confirm <token> --compact
```
