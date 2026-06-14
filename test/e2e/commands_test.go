package e2e

import (
	"strings"
	"testing"
)

// boundaryCase drives one leaf command through the real CLI boundary against
// the universal mock upstream. Write commands run their --dry-run step and
// must return a confirm token; high/critical writes carry --dangerous.
type boundaryCase struct {
	name  string
	args  []string
	write bool   // append --dry-run and expect data.confirm_token
	png   string // flag name that receives a generated PNG fixture
	file  string // "name.ext:content" written to the home dir, appended as positional arg
	flag  string // optional flag name that receives the file path instead of positional
}

var boundaryCases = []boundaryCase{
	// analytics (12 leaves share flags)
	{name: "analytics article summary", args: []string{"analytics", "article", "summary", "--begin-date", "2026-06-01", "--end-date", "2026-06-01"}},
	{name: "analytics article total", args: []string{"analytics", "article", "total", "--begin-date", "2026-06-01", "--end-date", "2026-06-01"}},
	{name: "analytics article read", args: []string{"analytics", "article", "read", "--begin-date", "2026-06-01", "--end-date", "2026-06-01"}},
	{name: "analytics article read-hour", args: []string{"analytics", "article", "read-hour", "--begin-date", "2026-06-01", "--end-date", "2026-06-01"}},
	{name: "analytics article share", args: []string{"analytics", "article", "share", "--begin-date", "2026-06-01", "--end-date", "2026-06-01"}},
	{name: "analytics article share-hour", args: []string{"analytics", "article", "share-hour", "--begin-date", "2026-06-01", "--end-date", "2026-06-01"}},
	{name: "analytics article published-summary", args: []string{"analytics", "article", "published-summary", "--begin-date", "2026-06-01", "--end-date", "2026-06-01"}},
	{name: "analytics article published-detail", args: []string{"analytics", "article", "published-detail", "--begin-date", "2026-06-01", "--end-date", "2026-06-01"}},
	{name: "analytics article published-read", args: []string{"analytics", "article", "published-read", "--begin-date", "2026-06-01", "--end-date", "2026-06-01"}},
	{name: "analytics article published-share", args: []string{"analytics", "article", "published-share", "--begin-date", "2026-06-01", "--end-date", "2026-06-01"}},
	{name: "analytics user summary", args: []string{"analytics", "user", "summary", "--begin-date", "2026-06-01", "--end-date", "2026-06-01"}},
	{name: "analytics user cumulate", args: []string{"analytics", "user", "cumulate", "--begin-date", "2026-06-01", "--end-date", "2026-06-01"}},

	// asset
	{name: "asset count", args: []string{"asset", "count"}},
	{name: "asset list", args: []string{"asset", "list", "--type", "news"}},
	{name: "asset get", args: []string{"asset", "get", "--media-id", "MEDIA_ID"}},
	{name: "asset delete", write: true, args: []string{"asset", "delete", "--media-id", "MEDIA_ID", "--dangerous"}},
	{name: "asset temp get", args: []string{"asset", "temp", "get", "--media-id", "MEDIA_ID"}},
	{name: "asset temp get-hd-voice", args: []string{"asset", "temp", "get-hd-voice", "--media-id", "MEDIA_ID"}},
	{name: "asset temp upload", write: true, png: "positional", args: []string{"asset", "temp", "upload", "--type", "image"}},

	// lifecycle / self-description
	{name: "changelog", args: []string{"changelog"}},
	{name: "context", args: []string{"context"}},
	{name: "doctor", args: []string{"doctor"}},
	{name: "reference", args: []string{"reference"}},
	{name: "update", args: []string{"update", "--check"}},

	// comment
	{name: "comment list", args: []string{"comment", "list", "--msg-data-id", "1"}},
	{name: "comment open", write: true, args: []string{"comment", "open", "--msg-data-id", "1", "--dangerous"}},
	{name: "comment close", write: true, args: []string{"comment", "close", "--msg-data-id", "1", "--dangerous"}},
	{name: "comment mark", write: true, args: []string{"comment", "mark", "--msg-data-id", "1", "--user-comment-id", "2"}},
	{name: "comment unmark", write: true, args: []string{"comment", "unmark", "--msg-data-id", "1", "--user-comment-id", "2"}},
	{name: "comment delete", write: true, args: []string{"comment", "delete", "--msg-data-id", "1", "--user-comment-id", "2", "--dangerous"}},
	{name: "comment reply-add", write: true, args: []string{"comment", "reply-add", "--msg-data-id", "1", "--user-comment-id", "2", "--content", "thanks", "--dangerous"}},
	{name: "comment reply-delete", write: true, args: []string{"comment", "reply-delete", "--msg-data-id", "1", "--user-comment-id", "2"}},

	// draft
	{name: "draft count", args: []string{"draft", "count"}},
	{name: "draft list", args: []string{"draft", "list"}},
	{name: "draft get", args: []string{"draft", "get", "--media-id", "MEDIA_ID"}},
	{name: "draft create", write: true, png: "cover-file", args: []string{
		"draft", "create", "--title", "T", "--content", "<p>hello</p>", "--dangerous"}},
	{name: "draft update", write: true, args: []string{
		"draft", "update", "--media-id", "MEDIA_ID", "--title", "T2", "--content", "<p>hi</p>",
		"--cover-media-id", "COVER_MEDIA", "--dangerous"}},
	{name: "draft delete", write: true, args: []string{"draft", "delete", "--media-id", "MEDIA_ID", "--dangerous"}},
	{name: "draft switch status", args: []string{"draft", "switch", "status"}},
	{name: "draft switch enable", write: true, args: []string{"draft", "switch", "enable", "--dangerous"}},

	// image
	{name: "image prepare", png: "positional", args: []string{"image", "prepare"}},
	{name: "image upload", write: true, png: "positional", args: []string{"image", "upload", "--type", "body"}},

	// menu
	{name: "menu get", args: []string{"menu", "get"}},
	{name: "menu set", write: true, flag: "file", file: `menu.json:{"button":[{"type":"view","name":"Home","url":"https://example.com"}]}`,
		args: []string{"menu", "set", "--dangerous"}},
	{name: "menu delete", write: true, args: []string{"menu", "delete", "--dangerous"}},
	{name: "menu addconditional", write: true, flag: "file",
		file: `cond.json:{"button":[{"type":"view","name":"Home","url":"https://example.com"}],"matchrule":{"tag_id":"2"}}`,
		args: []string{"menu", "addconditional", "--dangerous"}},

	// qrcode
	{name: "qrcode create", write: true, args: []string{"qrcode", "create", "--expire-seconds", "604800", "--scene-str", "promo-001"}},

	// user
	{name: "user info", args: []string{"user", "info", "OPENID"}},
	{name: "user list", args: []string{"user", "list"}},
	{name: "user info-batch", args: []string{"user", "info-batch", "--openids", "OPENID,OPENID2"}},

	// message mass (broadcast) — all writes; sendall/send/preview/delete are critical/high → --dangerous
	{name: "message mass sendall", write: true, args: []string{"message", "mass", "sendall", "--to-all", "--mpnews-media-id", "MEDIA_ID", "--dangerous"}},
	{name: "message mass send", write: true, args: []string{"message", "mass", "send", "--openids", "OPENID,OPENID2", "--mpnews-media-id", "MEDIA_ID", "--dangerous"}},
	{name: "message mass preview", write: true, args: []string{"message", "mass", "preview", "--openid", "OPENID", "--mpnews-media-id", "MEDIA_ID", "--dangerous"}},
	{name: "message mass get", args: []string{"message", "mass", "get", "--msg-id", "2247483647"}},
	{name: "message mass delete", write: true, args: []string{"message", "mass", "delete", "--msg-id", "2247483647", "--dangerous"}},

	// tag
	{name: "tag get", args: []string{"tag", "get"}},
	{name: "tag create", write: true, args: []string{"tag", "create", "--name", "VIP"}},
	{name: "tag update", write: true, args: []string{"tag", "update", "--id", "100", "--name", "VIP2"}},
	{name: "tag delete", write: true, args: []string{"tag", "delete", "--id", "100", "--dangerous"}},
	{name: "tag members", args: []string{"tag", "members", "100"}},
	{name: "tag tagging", write: true, args: []string{"tag", "tagging", "--id", "100", "--openid", "OPENID"}},
	{name: "tag untagging", write: true, args: []string{"tag", "untagging", "--id", "100", "--openid", "OPENID"}},

	// publish
	{name: "publish status", args: []string{"publish", "status", "--publish-id", "100000001"}},
	{name: "publish list", args: []string{"publish", "list"}},
	{name: "publish get-article", args: []string{"publish", "get-article", "--article-id", "ARTICLE_ID"}},
	{name: "publish submit", write: true, args: []string{"publish", "submit", "--media-id", "MEDIA_ID", "--dangerous"}},
	{name: "publish delete", write: true, args: []string{"publish", "delete", "--article-id", "ARTICLE_ID", "--dangerous"}},

	// render / remote / webhook
	{name: "render markdown", file: "doc.md:# Title\n\nbody", args: []string{"render", "markdown"}},
	{name: "render html", file: "doc.html:<h1>T</h1>", args: []string{"render", "html"}},
	{name: "remote ssh-command", args: []string{"remote", "ssh-command", "--host", "vps.example.com", "--user", "ops"}},
	{name: "webhook verify", args: []string{"webhook", "verify", "--token", "tok", "--timestamp", "1718000000", "--nonce", "n1", "--signature", "deadbeef", "--echostr", "hello"}},

	// setup
	{name: "setup account list", args: []string{"setup", "account", "list"}},
	{name: "setup account test", args: []string{"setup", "account", "test"}},
	{name: "setup account add", write: true, args: []string{
		"setup", "account", "add", "--alias", "main", "--app-id", "wx-new", "--secret", "s3cret"}},
	{name: "setup account default", write: true, args: []string{"setup", "account", "default", "--alias", "main"}},
	{name: "setup account remove", write: true, args: []string{"setup", "account", "remove", "--alias", "main"}},
	{name: "setup proxy status", args: []string{"setup", "proxy", "status"}},
	{name: "setup proxy set", write: true, args: []string{"setup", "proxy", "set", "--url", "http://127.0.0.1:7890"}},
	{name: "setup proxy clear", write: true, args: []string{"setup", "proxy", "clear"}},

	// token
	{name: "token status", args: []string{"token", "status"}},
	{name: "token refresh", args: []string{"token", "refresh"}},
}

func TestBoundary_AllLeafCommands(t *testing.T) {
	for _, tc := range boundaryCases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			args := append([]string{"--format", "json"}, tc.args...)
			if tc.png != "" {
				path := writePNG(t, home)
				if tc.png == "positional" {
					args = append(args, path)
				} else {
					args = append(args, "--"+tc.png, path)
				}
			}
			if tc.file != "" {
				parts := strings.SplitN(tc.file, ":", 2)
				path := writeFile(t, home, parts[0], parts[1])
				if tc.flag != "" {
					args = append(args, "--"+tc.flag, path)
				} else {
					args = append(args, path)
				}
			}
			if tc.write {
				args = append(args, "--dry-run")
			}
			r := runCLI(home, nil, args...)
			data := decodeOK(t, r)
			if tc.write {
				token, _ := data["confirm_token"].(string)
				if strings.TrimSpace(token) == "" {
					t.Fatalf("dry-run should return a confirm_token, got: %s", r.Stdout)
				}
			}
		})
	}
}

// TestBoundary_DryRunConfirmCycle: full two-step write loop across processes
// sharing one HOME (the HMAC confirm.secret must persist).
func TestBoundary_DryRunConfirmCycle(t *testing.T) {
	home := t.TempDir()
	dry := runCLI(home, nil, "--format", "json", "comment", "mark", "--msg-data-id", "7", "--user-comment-id", "9", "--dry-run")
	data := decodeOK(t, dry)
	token, _ := data["confirm_token"].(string)
	if strings.TrimSpace(token) == "" {
		t.Fatalf("no confirm_token in dry-run: %s", dry.Stdout)
	}
	run := runCLI(home, nil, "--format", "json", "comment", "mark", "--msg-data-id", "7", "--user-comment-id", "9", "--confirm", token)
	decodeOK(t, run)
}

func TestBoundary_DangerousGateBlocksWithoutFlag(t *testing.T) {
	r := runCLI(t.TempDir(), nil, "--format", "json", "publish", "submit", "--media-id", "MEDIA_ID", "--dry-run")
	if r.ExitCode != 5 {
		t.Fatalf("exit code = %d, want 5\nstdout: %s", r.ExitCode, r.Stdout)
	}
	env := decodeEnvelope(t, r.Stdout)
	if env.OK || env.Error == nil || !strings.Contains(env.Error.Message, "--dangerous") {
		t.Fatalf("want dangerous-gate error envelope, got: %s", r.Stdout)
	}
}

func TestBoundary_UntrustedTaggingOnCommentList(t *testing.T) {
	r := runCLI(t.TempDir(), nil, "--format", "json", "comment", "list", "--msg-data-id", "1")
	data := decodeOK(t, r)
	tagged, _ := data["_untrusted"].([]any)
	if len(tagged) == 0 || tagged[0] != "comment" {
		t.Fatalf("comment list should tag the comment subtree as _untrusted, got: %s", r.Stdout)
	}
}

func TestBoundary_TokenCachedAcrossInvocations(t *testing.T) {
	home := t.TempDir()
	decodeOK(t, runCLI(home, nil, "--format", "json", "draft", "count"))
	status := decodeOK(t, runCLI(home, nil, "--format", "json", "token", "status"))
	if status["cached"] != true || status["valid"] != true {
		t.Fatalf("token should be cached and valid after an API call, got: %v", status)
	}
}

func TestBoundary_MissingCredentialsIsConfigError(t *testing.T) {
	r := runCLI(t.TempDir(), map[string]string{
		"WECHAT_MP_CLI_APP_ID": "", "WECHAT_MP_CLI_APP_SECRET": "",
	}, "--format", "json", "draft", "count")
	if r.ExitCode == 0 {
		t.Fatalf("missing credentials should fail\nstdout: %s", r.Stdout)
	}
	env := decodeEnvelope(t, r.Stdout)
	if env.OK || env.Error == nil {
		t.Fatalf("want error envelope on stdout, got: %s", r.Stdout)
	}
}

func TestBoundary_MissingRequiredFlagIsValidationError(t *testing.T) {
	r := runCLI(t.TempDir(), nil, "--format", "json", "comment", "list")
	if r.ExitCode != 2 {
		t.Fatalf("exit code = %d, want 2\nstdout: %s", r.ExitCode, r.Stdout)
	}
}
