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

## 2026-06-15 — batch commands (real credentials, live probe)

Re-run of the batch surface **with real test-account credentials present** this
time (supplied via `WECHAT_MP_CLI_APP_ID` / `WECHAT_MP_CLI_APP_SECRET` env vars
only — never written to any file, commit, or this document). Built from branch
`smoke/batch-2026-06-15` source (`go build`); the globally installed v1.0.2 has
no batch commands.

- **Account state:** `user list` → `total: 0`, `next_openid: ""`. The test
  account still has **zero followers**, so follower-dependent reads have no live
  data and broadcasts have no recipients (non-polluting by construction).
- **AppID/AppSecret are intentionally not recorded.** Only aggregate pass/fail,
  WeChat return codes, and exit codes — no openids, msg_ids, tokens, or secrets.

### Live API probe results (real round-trips)

Each command was driven to a **real** `api.weixin.qq.com` call to observe the
genuine upstream return code per the test account's permission grant.

| Command | Classification | Live result |
|---|---|---|
| `user info-batch --openids` | **live** | Real batchget call; invalid openids → upstream `40003` mapped per-item to `E_VALIDATION`; `items[]`/`summary` aggregated correctly (see below). |
| `message mass sendall --to-all` | **live, permission OK** | Real call returned `errcode:0` "send job submission success" + a sandbox `msg_id`. Test account **has** sendall permission; 0 followers → no recipients reached. |
| `message mass send --openids` | **live, permission restricted (48001)** | Real call returned WeChat `48001 api unauthorized` → `E_FORBIDDEN`, exit 4. **Openid-list mass send is not authorized on the test account.** |
| `message mass preview` | **live, permission OK** | Real call returned `40003 invalid openid` (not 48001) → `E_VALIDATION`. The API accepted the request and only rejected the synthetic openid; preview permission is present. |
| `message mass get --msg-id` | **live** | Real call with a bogus id returned `40059 invalid msg id` → `E_VALIDATION`. Read permission present. |
| `message mass delete --msg-id` | **live** | Real call with a bogus id returned `40059 invalid msg id` → `E_VALIDATION`. Delete reached real API; only the bogus id was rejected. No real message recalled. |

**Permission summary:** on this test account only the **openid-list `mass send`**
returned `48001 api unauthorized`. `sendall`, `preview`, `get`, and `delete` all
reached genuine API behavior (`errcode:0` or content-level errors), so they are
**not** permission-restricted here.

### user info-batch aggregation (live)

Input `--openids o_fake_BBB,o_fake_AAA,o_fake_BBB --openids o_fake_AAA,o_fake_CCC`
resolved to exactly `[BBB, AAA, CCC]` — **de-duplicated, input order preserved**.
All three failed at the live batchget call (`40003`); the chunk-level error
mapped onto **every item** as `E_VALIDATION`, `summary{total:3,succeeded:0,failed:3}`
(counts equal the item tally), and the envelope carried `_untrusted:["items"]`.
The follower-success path and `E_NOT_FOUND` ghost-item path remain mock-verified
only (no real followers to exercise them).

### Guards & gating (real creds present, so gates run for real)

| Contract point | Result | Method |
|---|---|---|
| `--dangerous` second gate (critical `sendall`) | PASS | Without `--dangerous`: `E_CONFIRMATION_REQUIRED`, exit 5. With `--dangerous --dry-run`: confirm token + `expires_at` issued. |
| Single-use confirm replay | PASS | `send` confirm token consumed by first call (token burned **before** the write, so even the 48001 failure consumed it); replay → `E_CONFLICT`, exit 6. |
| Audience guard — neither `--to-all` nor `--tag-id` | PASS | `E_VALIDATION` "set --to-all or a positive --tag-id", exit 2. |
| Audience guard — `--to-all` **and** `--tag-id` together | **OBSERVATION / gap** | **Not rejected.** `--to-all` silently wins (`audience:"all followers"`); the command does not enforce mutual exclusion when both are set (`message.go` only requires a tag when `!toAll`). Expected a usage error per the mutual-exclusion intent; recorded honestly as a gap, not a pass. |
| Body guard — no body | PASS | `E_VALIDATION` "a message body is required", exit 2. |
| Body guard — both `--text` and `--mpnews-media-id` | PASS | `E_VALIDATION` "set only one of …", exit 2. |
| `mass get` / `info-batch` validation precedence | PASS | `--msg-id 0` and empty `--openids` → `E_VALIDATION` before any API call. |
| Dry-run envelopes (all batch cmds) | PASS | `send`/`preview`/`delete`/`sendall` dry-run all emit `operation` + `preview` + `confirm_token` + `expires_at`; `send` preview shows de-duped `targets[]` + `total`. |

### Honest classification per command

| Command | Grade |
|---|---|
| `user info-batch` | **live** (validation + per-item aggregation against real API; success path mock-only) |
| `message mass sendall` | **live**, permission OK (0 followers → no recipients) |
| `message mass send` | **live**, **permission restricted (48001)** |
| `message mass preview` | **live**, permission OK (only synthetic openid rejected) |
| `message mass get` | **live** |
| `message mass delete` | **live** (real API reached; only bogus id rejected) |

One gap found this round: `mass sendall` does not reject `--to-all` + `--tag-id`
supplied together (see guards table). No test account pollution occurred.

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
