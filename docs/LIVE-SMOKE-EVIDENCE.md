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

## 2026-06-15 — batch commands (dry-run / contract only)

New batch surface: `message mass sendall` / `mass send --openids` / `mass preview`
/ `mass get` / `mass delete`, and `user info-batch`.

**Environment for this round: no live WeChat credentials were available**
(`doctor` → `account_config: warn (no account configured)`, no
`WECHAT_MP_CLI_APP_ID`/`_APP_SECRET`), and per the safety policy
broadcast/irreversible batches are **dry-run only at night** regardless. So no
real broadcast, preview-send, recall, or follower read was executed against the
live API. Verification was done **offline against the built binary** (validation
/ gate / envelope / single-use confirm) plus the **mock-upstream e2e suite**
(per-item aggregation, replay-conflict through a real send to the mock).
Aggregate pass/fail only — no openids, msg_ids, tokens, or secrets recorded.

| Command | Result | Method |
|---|---|---|
| `message mass sendall` | dry-run only | binary: critical `--dangerous` double-gate enforced; `--dry-run` issues a confirm token (not executed); audience guard (`--to-all` xor positive `--tag-id`) and body guard (exactly one of `--text`/`--mpnews-media-id`) both return `E_VALIDATION`. **Not real-machine executed** (no creds + night policy). |
| `message mass send --openids` | dry-run only | binary: critical gate enforced; plural `--openids` (comma + repeated) de-duped, input order preserved, blast-radius `total`/`targets` shown in preview; empty list and dual-body → `E_VALIDATION`. **Not real-machine executed.** |
| `message mass preview` | dry-run only | binary: high-risk gate enforced; `--dry-run` confirm token issued; missing recipient (`--openid`/`--towxname`) → `E_VALIDATION`. Sends one real message when confirmed, so **not executed** (no creds). |
| `message mass delete` | dry-run only | binary: critical gate enforced; `--dry-run` token issued (not executed); `--msg-id ≤ 0` → `E_VALIDATION`. Irreversible recall → **dry-run only, never real-machine executed.** |
| `message mass get` | not tested (live) | binary: `--msg-id ≤ 0` → `E_VALIDATION` before any call; valid id with no creds → `E_CONFIG` (validation precedence confirmed). Live read **not tested** — needs a real `msg_id` + credentials. Read path mock-verified. |
| `user info-batch` | dry/structure only | binary: empty `--openids` → `E_VALIDATION`; populated list with no creds → `E_CONFIG` (validation + plural parse precede the call). Per-item aggregation (`items[]` keyed by openid, de-dupe, `_untrusted`, `summary` total/succeeded/failed) **mock-verified** in e2e. Live read **not tested** — needs real followers. |

### Contract points exercised this round

| Contract point | Result | Method |
|---|---|---|
| Single-use confirm token (replay rejected) | PASS | binary: dry-run → first `--confirm` consumes token (then `E_CONFIG`, no creds) → replay returns `E_CONFLICT` "token already used". Also mock e2e `TestBatch_MassSendConfirmThenReplayConflict`. |
| `--dangerous` second gate (high/critical) | PASS | binary: every critical (`sendall`/`send`/`delete`) and high (`preview`) command blocks with `E_CONFIRMATION_REQUIRED` when `--dangerous` is absent. |
| Partial-failure aggregation shape | PASS | mock e2e `TestBatch_InfoBatchAggregatesPerItem`: 3 inputs incl. one non-follower → `succeeded:2 / failed:1`, item order preserved, ghost item carries `E_NOT_FOUND`. |

No real broadcast, recall, preview-send, or follower read was performed. The
irreversible/broadcast batches (`mass sendall`, `mass send`, `mass delete`) were
**dry-run verified, never real-machine executed**, per the safety policy.

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
