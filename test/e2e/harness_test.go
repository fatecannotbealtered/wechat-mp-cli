package e2e

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var (
	binaryPath string
	mockURL    string
)

// universalBody satisfies the common WeChat API response shapes: errcode/errmsg
// success plus the list/item fields the command parsers read. Path overrides
// below handle endpoints that need a specific shape.
const universalBody = `{
  "errcode": 0,
  "errmsg": "ok",
  "total_count": 0,
  "item_count": 0,
  "item": [],
  "comment": [],
  "news_item": [],
  "media_id": "MEDIA_ID",
  "url": "https://mmbiz.example/img",
  "publish_id": "100000001",
  "msg_data_id": 1,
  "article_id": "ARTICLE_ID",
  "is_open": 1
}`

var pathOverrides = map[string]string{
	"/cgi-bin/stable_token": `{"access_token":"test-access-token","expires_in":7200}`,
}

func universalMux() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		best := ""
		for prefix := range pathOverrides {
			if strings.HasPrefix(r.URL.Path, prefix) && len(prefix) > len(best) {
				best = prefix
			}
		}
		if best != "" {
			_, _ = w.Write([]byte(pathOverrides[best]))
			return
		}
		_, _ = w.Write([]byte(universalBody))
	})
}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "wechat-mp-e2e-*")
	if err != nil {
		panic("temp dir: " + err.Error())
	}

	name := "wechat-mp-cli"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binaryPath = filepath.Join(dir, name)

	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/wechat-mp-cli")
	build.Dir = projectRoot()
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic("build binary: " + err.Error())
	}

	server := httptest.NewServer(universalMux())
	mockURL = server.URL

	code := m.Run()
	server.Close()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func projectRoot() string {
	_, src, _, _ := runtime.Caller(0)
	dir := filepath.Dir(src)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	wd, _ := os.Getwd()
	return wd
}

type cmdResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type jsonEnvelope struct {
	OK            bool            `json:"ok"`
	SchemaVersion string          `json:"schema_version"`
	Data          json.RawMessage `json:"data"`
	Error         *jsonError      `json:"error"`
	Meta          map[string]any  `json:"meta"`
}

type jsonError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details"`
	Retryable bool           `json:"retryable"`
}

// runCLI executes the built binary against the universal mock with an
// isolated HOME (config, confirm.secret, token cache). The same home must be
// shared between a dry-run and its confirm step.
func runCLI(home string, extraEnv map[string]string, args ...string) cmdResult {
	cmd := exec.Command(binaryPath, args...)
	drop := map[string]bool{
		"WECHAT_MP_CLI_API_BASE": true, "WECHAT_MP_CLI_APP_ID": true,
		"WECHAT_MP_CLI_APP_SECRET": true, "WECHAT_MP_CLI_ACCOUNT": true,
		"WECHAT_APP_ID": true, "WECHAT_APP_SECRET": true,
		"WECHAT_APPID": true, "WECHAT_APPSECRET": true,
		"HOME": true, "USERPROFILE": true,
	}
	env := []string{}
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if !drop[key] {
			env = append(env, kv)
		}
	}
	env = append(env,
		"WECHAT_MP_CLI_API_BASE="+mockURL,
		"WECHAT_MP_CLI_APP_ID=wx-test-app",
		"WECHAT_MP_CLI_APP_SECRET=test-secret",
		"HOME="+home,
		"USERPROFILE="+home,
	)
	for k, v := range extraEnv {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		exitCode = -1
	}
	return cmdResult{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: exitCode}
}

func decodeEnvelope(t *testing.T, stdout string) jsonEnvelope {
	t.Helper()
	var env jsonEnvelope
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &env); err != nil {
		t.Fatalf("invalid JSON envelope: %v\nstdout: %s", err, stdout)
	}
	if env.SchemaVersion == "" {
		t.Fatalf("missing schema_version: %s", stdout)
	}
	return env
}

func decodeOK(t *testing.T, r cmdResult) map[string]any {
	t.Helper()
	if r.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
	}
	env := decodeEnvelope(t, r.Stdout)
	if !env.OK {
		t.Fatalf("ok=false: %s", r.Stdout)
	}
	var data map[string]any
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, &data); err != nil {
			var arr []any
			if err2 := json.Unmarshal(env.Data, &arr); err2 != nil {
				t.Fatalf("envelope data is neither object nor array: %v\n%s", err, r.Stdout)
			}
		}
	}
	return data
}

// --- fixtures ---

func writePNG(t *testing.T, dir string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	path := filepath.Join(dir, "cover.png")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write png: %v", err)
	}
	return path
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}
