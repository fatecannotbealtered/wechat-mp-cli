package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fatecannotbealtered/wechat-mp-cli/internal/output"
)

// runUpdateCapture drives the update command through the real CLI boundary
// (rootCmd) with the given args, capturing the single JSON envelope on stdout.
// It resets the update-related global flags before each run.
func runUpdateCapture(t *testing.T, args ...string) (map[string]any, int) {
	t.Helper()
	updateCheck = false
	updateTargetVersion = ""
	dryRun = false
	confirmFlag = ""
	forceMode = false
	compactJSON = false
	formatMode = formatJSON

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	origStdout := os.Stdout
	os.Stdout = w

	rootCmd.SetArgs(args)
	execErr := ExecuteContext(context.Background())
	_ = w.Close()
	os.Stdout = origStdout

	out, _ := io.ReadAll(r)
	_ = r.Close()

	var env map[string]any
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("output is not a single JSON envelope (%v): %s", err, out)
	}
	exit := LastExitCode()
	_ = execErr
	return env, exit
}

func envData(t *testing.T, env map[string]any) map[string]any {
	t.Helper()
	data, _ := env["data"].(map[string]any)
	if data == nil {
		t.Fatalf("envelope has no data: %v", env)
	}
	return data
}

func envError(t *testing.T, env map[string]any) map[string]any {
	t.Helper()
	e, _ := env["error"].(map[string]any)
	if e == nil {
		t.Fatalf("envelope has no error: %v", env)
	}
	return e
}

// stubUpdateSuccess wires every update seam to a no-network happy path and
// returns a restore func.
func stubUpdateSeams(t *testing.T) func() {
	t.Helper()
	origExe := updateBinaryExecutable
	origApply := updateBinaryApply
	origSync := updateSkillSync
	origVerify := updateVerifySignature
	updateBinaryExecutable = func() (string, error) { return "/tmp/wechat-mp-cli", nil }
	updateBinaryApply = func(_, dst string) (updateApplyResult, error) {
		return updateApplyResult{Status: "installed", Path: dst}, nil
	}
	updateSkillSync = func(_ context.Context, _ string) error { return nil }
	updateVerifySignature = func(_, _, _ string) error { return nil }
	return func() {
		updateBinaryExecutable = origExe
		updateBinaryApply = origApply
		updateSkillSync = origSync
		updateVerifySignature = origVerify
	}
}

// TestUpdate_BareRunsWithoutToken proves the confirm gate is gone: a bare
// `update` does NOT return E_CONFIRMATION_REQUIRED. We point it at an
// unreachable API so it fails at the discover stage, but the failure must be a
// network error (gate removed), never a missing-token error.
func TestUpdate_BareRunsWithoutToken(t *testing.T) {
	origAPI := updateBinaryGitHubAPI
	updateBinaryGitHubAPI = "http://127.0.0.1:0"
	origExe := updateBinaryExecutable
	updateBinaryExecutable = func() (string, error) { return "/tmp/wechat-mp-cli", nil }
	defer func() { updateBinaryGitHubAPI = origAPI; updateBinaryExecutable = origExe }()

	env, exit := runUpdateCapture(t, "update")
	if ok, _ := env["ok"].(bool); ok {
		t.Fatalf("bare update against unreachable API should fail, got ok=true: %v", env)
	}
	e := envError(t, env)
	if e["code"] == "E_CONFIRMATION_REQUIRED" {
		t.Fatalf("bare update must not require a confirm token anymore: %v", e)
	}
	if e["code"] != "E_NETWORK" {
		t.Fatalf("discover failure should be E_NETWORK, got %v", e["code"])
	}
	if exit != ExitRetryable {
		t.Fatalf("network failure exit = %d, want %d", exit, ExitRetryable)
	}
	details, _ := e["details"].(map[string]any)
	if details["stage"] != "discover" || details["binary_replaced"] != false {
		t.Fatalf("failure envelope must carry stage/binary_replaced: %v", details)
	}
}

// TestUpdate_DryRunNoToken: --dry-run is a read-only preview that issues NO
// confirm_token and NO expires_at.
func TestUpdate_DryRunNoToken(t *testing.T) {
	env, exit := runUpdateCapture(t, "update", "--dry-run")
	if ok, _ := env["ok"].(bool); !ok {
		t.Fatalf("dry-run should succeed read-only: %v", env)
	}
	if exit != ExitOK {
		t.Fatalf("dry-run exit = %d, want 0", exit)
	}
	data := envData(t, env)
	if _, ok := data["confirm_token"]; ok {
		t.Fatalf("dry-run must not issue confirm_token: %v", data)
	}
	if _, ok := data["expires_at"]; ok {
		t.Fatalf("dry-run must not issue expires_at: %v", data)
	}
	if data["action"] == nil || data["changes"] == nil {
		t.Fatalf("dry-run should preview the plan: %v", data)
	}
}

// TestUpdate_IntegrityFailureNonRetryable: a signature/checksum failure is
// E_INTEGRITY, exit 1, retryable:false — and must not be weakened.
func TestUpdate_IntegrityFailureNonRetryable(t *testing.T) {
	restore := stubUpdateSeams(t)
	defer restore()
	// Force the binary update to fail at verify_signature.
	updateVerifySignature = func(_, _, _ string) error { return errors.New("certificate identity mismatch") }

	srv := newUpdateReleaseServer(t)
	defer srv.close()
	origAPI := updateBinaryGitHubAPI
	origClient := updateBinaryHTTPClient
	updateBinaryGitHubAPI = srv.api
	updateBinaryHTTPClient = srv.client
	defer func() { updateBinaryGitHubAPI = origAPI; updateBinaryHTTPClient = origClient }()

	env, exit := runUpdateCapture(t, "update")
	e := envError(t, env)
	if e["code"] != "E_INTEGRITY" {
		t.Fatalf("integrity failure code = %v, want E_INTEGRITY", e["code"])
	}
	if e["retryable"] != false {
		t.Fatalf("E_INTEGRITY must be non-retryable: %v", e)
	}
	if exit != ExitError {
		t.Fatalf("E_INTEGRITY exit = %d, want 1", exit)
	}
	details, _ := e["details"].(map[string]any)
	if details["stage"] != "verify_signature" || details["binary_replaced"] != false {
		t.Fatalf("integrity envelope stage/binary_replaced wrong: %v", details)
	}
}

// TestUpdate_SkillSyncFailureIsPartialSuccess: after a successful binary swap, a
// skill-sync failure is partial success (ok:false, binary_replaced:true) with a
// retryable skill_sync_command — not a hard network error.
func TestUpdate_SkillSyncFailureIsPartialSuccess(t *testing.T) {
	restore := stubUpdateSeams(t)
	defer restore()
	updateSkillSync = func(_ context.Context, _ string) error { return errors.New("npx not found") }

	srv := newUpdateReleaseServer(t)
	defer srv.close()
	origAPI := updateBinaryGitHubAPI
	origClient := updateBinaryHTTPClient
	updateBinaryGitHubAPI = srv.api
	updateBinaryHTTPClient = srv.client
	defer func() { updateBinaryGitHubAPI = origAPI; updateBinaryHTTPClient = origClient }()

	env, exit := runUpdateCapture(t, "update")
	if ok, _ := env["ok"].(bool); ok {
		t.Fatalf("skill-sync failure must not report ok:true: %v", env)
	}
	e := envError(t, env)
	details, _ := e["details"].(map[string]any)
	if details["binary_replaced"] != true {
		t.Fatalf("partial success must report binary_replaced:true: %v", details)
	}
	if details["stage"] != "skill_sync" {
		t.Fatalf("partial success stage = %v, want skill_sync", details["stage"])
	}
	if e["retryable"] != true {
		t.Fatalf("skill-sync failure should be retryable: %v", e)
	}
	if _, ok := details["skill_sync_command"]; !ok {
		t.Fatalf("partial success must carry skill_sync_command: %v", details)
	}
	if exit != ExitError {
		t.Fatalf("skill-sync partial exit = %d, want 1", exit)
	}
}

// TestUpdate_ReplaceIOFailureIsEIO: a local replace-stage io failure maps to
// E_IO (exit 1), not E_NETWORK.
func TestUpdate_ReplaceIOFailureIsEIO(t *testing.T) {
	restore := stubUpdateSeams(t)
	defer restore()
	updateBinaryApply = func(_, _ string) (updateApplyResult, error) {
		return updateApplyResult{}, errors.New("disk full")
	}

	srv := newUpdateReleaseServer(t)
	defer srv.close()
	origAPI := updateBinaryGitHubAPI
	origClient := updateBinaryHTTPClient
	updateBinaryGitHubAPI = srv.api
	updateBinaryHTTPClient = srv.client
	defer func() { updateBinaryGitHubAPI = origAPI; updateBinaryHTTPClient = origClient }()

	env, exit := runUpdateCapture(t, "update")
	e := envError(t, env)
	if e["code"] != "E_IO" {
		t.Fatalf("replace io failure code = %v, want E_IO", e["code"])
	}
	if exit != ExitError {
		t.Fatalf("E_IO exit = %d, want 1", exit)
	}
	if e["retryable"] != false {
		t.Fatalf("E_IO must be non-retryable: %v", e)
	}
	details, _ := e["details"].(map[string]any)
	if details["stage"] != "replace" || details["binary_replaced"] != false {
		t.Fatalf("replace failure envelope wrong: %v", details)
	}
}

// TestUpdate_ReplacePermissionFailureIsForbidden: a permission-denied replace
// maps to E_FORBIDDEN (exit 4).
func TestUpdate_ReplacePermissionFailureIsForbidden(t *testing.T) {
	restore := stubUpdateSeams(t)
	defer restore()
	updateBinaryApply = func(_, _ string) (updateApplyResult, error) {
		return updateApplyResult{}, os.ErrPermission
	}

	srv := newUpdateReleaseServer(t)
	defer srv.close()
	origAPI := updateBinaryGitHubAPI
	origClient := updateBinaryHTTPClient
	updateBinaryGitHubAPI = srv.api
	updateBinaryHTTPClient = srv.client
	defer func() { updateBinaryGitHubAPI = origAPI; updateBinaryHTTPClient = origClient }()

	env, exit := runUpdateCapture(t, "update")
	e := envError(t, env)
	if e["code"] != "E_FORBIDDEN" {
		t.Fatalf("permission replace failure code = %v, want E_FORBIDDEN", e["code"])
	}
	if exit != ExitAuth {
		t.Fatalf("E_FORBIDDEN exit = %d, want 4", exit)
	}
}

// TestUpdate_Success: full happy path returns ok with previous/current version,
// binary_replaced:true, and synced skill.
func TestUpdate_Success(t *testing.T) {
	restore := stubUpdateSeams(t)
	defer restore()

	srv := newUpdateReleaseServer(t)
	defer srv.close()
	origAPI := updateBinaryGitHubAPI
	origClient := updateBinaryHTTPClient
	updateBinaryGitHubAPI = srv.api
	updateBinaryHTTPClient = srv.client
	defer func() { updateBinaryGitHubAPI = origAPI; updateBinaryHTTPClient = origClient }()

	env, exit := runUpdateCapture(t, "update")
	if ok, _ := env["ok"].(bool); !ok {
		t.Fatalf("happy path should be ok:true: %v", env)
	}
	if exit != ExitOK {
		t.Fatalf("success exit = %d, want 0", exit)
	}
	data := envData(t, env)
	if data["binary_replaced"] != true {
		t.Fatalf("success must report binary_replaced:true: %v", data)
	}
	if data["skill_sync_status"] != "synced" {
		t.Fatalf("success skill_sync_status = %v, want synced", data["skill_sync_status"])
	}
	if data["signature_verified"] != true {
		t.Fatalf("success signature_verified = %v, want true", data["signature_verified"])
	}
	if !strings.Contains(toStr(data["next_step"]), "changelog --since") {
		t.Fatalf("success should hint changelog: %v", data["next_step"])
	}
}

// TestUpdate_InterruptBeforeSwap: a cancelled context before the swap emits the
// terminal E_INTERRUPTED envelope (exit 130) stating no change.
func TestUpdate_InterruptBeforeSwap(t *testing.T) {
	skill := updateSkillSyncCommand()
	// Drive the helper directly with a cancelled context: it must report the
	// no-change interruption envelope on stdout.
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	lastExit = 0
	formatMode = formatJSON
	jsonMode = true
	output.Quiet = false
	_ = reportUpdateInterrupted(updateStageDownload, version, false, skill)
	_ = w.Close()
	os.Stdout = origStdout
	out, _ := io.ReadAll(r)
	_ = r.Close()

	var env map[string]any
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("interrupt output not JSON (%v): %s", err, out)
	}
	e := envError(t, env)
	if e["code"] != "E_INTERRUPTED" {
		t.Fatalf("interrupt code = %v, want E_INTERRUPTED", e["code"])
	}
	if e["retryable"] != true {
		t.Fatalf("interrupt should be retryable: %v", e)
	}
	if LastExitCode() != ExitInterrupted {
		t.Fatalf("interrupt exit = %d, want 130", LastExitCode())
	}
	if !strings.Contains(toStr(e["message"]), "no change") {
		t.Fatalf("pre-swap interrupt message must state no change: %v", e["message"])
	}
	details, _ := e["details"].(map[string]any)
	if details["binary_replaced"] != false {
		t.Fatalf("pre-swap interrupt binary_replaced must be false: %v", details)
	}
}

// TestApplyUpdateBinary_RenameSwap exercises the real cross-platform rename
// trick (no GOOS branch): the running target is replaced in place, the new
// bytes land, the status is "installed" (never "scheduled"), and the .new/.old
// scratch siblings are cleaned up. This is the in-process atomic swap that
// replaced the old Windows .cmd defer mechanism.
func TestApplyUpdateBinary_RenameSwap(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "wechat-mp-cli")
	if err := os.WriteFile(dst, []byte("OLD-BINARY"), 0o755); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	src := filepath.Join(dir, "extracted-wechat-mp-cli")
	if err := os.WriteFile(src, []byte("NEW-BINARY"), 0o755); err != nil {
		t.Fatalf("seed src: %v", err)
	}

	res, err := applyUpdateBinary(src, dst)
	if err != nil {
		t.Fatalf("applyUpdateBinary: %v", err)
	}
	if res.Status != "installed" {
		t.Fatalf("status = %q, want installed", res.Status)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != "NEW-BINARY" {
		t.Fatalf("target bytes = %q, want NEW-BINARY", got)
	}
	for _, scratch := range []string{
		filepath.Join(dir, ".wechat-mp-cli.new"),
		filepath.Join(dir, ".wechat-mp-cli.old"),
	} {
		if _, err := os.Stat(scratch); !os.IsNotExist(err) {
			t.Fatalf("scratch file %s should be cleaned up, stat err = %v", scratch, err)
		}
	}
}

// TestUpdate_BundleDownloadFailureIsRetryable proves Fix #1: when the signature
// BUNDLE itself cannot be downloaded (transport/HTTP failure), that is a
// transient network problem at the download stage — NOT a non-retryable
// E_INTEGRITY forged-release verdict. The release advertises a bundle asset, but
// the bundle URL 500s; the verify-signature stub never runs.
func TestUpdate_BundleDownloadFailureIsRetryable(t *testing.T) {
	restore := stubUpdateSeams(t)
	defer restore()
	// If this stub were reached, the test would wrongly pass as integrity; make
	// it loudly wrong so a regression that routes here is visible.
	updateVerifySignature = func(_, _, _ string) error {
		t.Fatalf("verify must not run when the bundle download itself fails")
		return nil
	}

	srv := newUpdateReleaseServer(t)
	defer srv.close()
	// Make only the bundle endpoint fail with a 5xx.
	srv.bundleStatus = http.StatusInternalServerError

	origAPI := updateBinaryGitHubAPI
	origClient := updateBinaryHTTPClient
	updateBinaryGitHubAPI = srv.api
	updateBinaryHTTPClient = srv.client
	defer func() { updateBinaryGitHubAPI = origAPI; updateBinaryHTTPClient = origClient }()

	env, exit := runUpdateCapture(t, "update")
	e := envError(t, env)
	if e["code"] == "E_INTEGRITY" {
		t.Fatalf("bundle DOWNLOAD failure must not be E_INTEGRITY: %v", e)
	}
	if e["code"] != "E_SERVER" {
		t.Fatalf("bundle download 5xx code = %v, want E_SERVER", e["code"])
	}
	if e["retryable"] != true {
		t.Fatalf("bundle download failure must be retryable: %v", e)
	}
	if exit != ExitRetryable {
		t.Fatalf("bundle download 5xx exit = %d, want %d", exit, ExitRetryable)
	}
	details, _ := e["details"].(map[string]any)
	if details["stage"] != "download" {
		t.Fatalf("bundle download failure stage = %v, want download", details["stage"])
	}
	if details["binary_replaced"] != false {
		t.Fatalf("download-stage failure must leave binary untouched: %v", details)
	}
}

// TestUpdate_DiscoverHTTPStatusClassification proves Fix #2: a discover-stage
// HTTP failure is mapped by STATUS, not collapsed into E_NETWORK. A 404 (e.g. an
// unknown --target-version) is non-retryable E_NOT_FOUND; a 429 is retryable
// E_RATE_LIMITED; a 5xx is retryable E_SERVER.
func TestUpdate_DiscoverHTTPStatusClassification(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		wantCode  string
		wantExit  int
		wantRetry bool
	}{
		{"not_found", http.StatusNotFound, "E_NOT_FOUND", ExitNotFound, false},
		{"rate_limited", http.StatusTooManyRequests, "E_RATE_LIMITED", ExitRetryable, true},
		{"server_5xx", http.StatusBadGateway, "E_SERVER", ExitRetryable, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			origExe := updateBinaryExecutable
			updateBinaryExecutable = func() (string, error) { return "/tmp/wechat-mp-cli", nil }
			defer func() { updateBinaryExecutable = origExe }()

			mux := http.NewServeMux()
			srv := httptest.NewServer(mux)
			defer srv.Close()
			mux.HandleFunc("/repos/", func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "boom", tc.status)
			})

			origAPI := updateBinaryGitHubAPI
			origClient := updateBinaryHTTPClient
			updateBinaryGitHubAPI = srv.URL
			updateBinaryHTTPClient = srv.Client()
			defer func() { updateBinaryGitHubAPI = origAPI; updateBinaryHTTPClient = origClient }()

			env, exit := runUpdateCapture(t, "update")
			e := envError(t, env)
			if e["code"] != tc.wantCode {
				t.Fatalf("discover %d code = %v, want %v", tc.status, e["code"], tc.wantCode)
			}
			if exit != tc.wantExit {
				t.Fatalf("discover %d exit = %d, want %d", tc.status, exit, tc.wantExit)
			}
			if e["retryable"] != tc.wantRetry {
				t.Fatalf("discover %d retryable = %v, want %v", tc.status, e["retryable"], tc.wantRetry)
			}
			details, _ := e["details"].(map[string]any)
			if details["stage"] != "discover" {
				t.Fatalf("discover %d stage = %v, want discover", tc.status, details["stage"])
			}
		})
	}
}

// TestDetectInstallMethod proves Fix #3: the install method is probed from the
// real executable path / env override, not hardcoded. A binary running out of a
// node_modules tree is npm; otherwise github-binary; an explicit env override
// wins.
func TestDetectInstallMethod(t *testing.T) {
	origExe := updateBinaryExecutable
	origEnv := updateGetenv
	defer func() { updateBinaryExecutable = origExe; updateGetenv = origEnv }()

	cases := []struct {
		name string
		exe  string
		env  string
		want string
	}{
		{"npm_node_modules", "/home/u/.npm/lib/node_modules/@fateforge/wechat-mp-cli-linux-x64/bin/wechat-mp-cli", "", "npm"},
		{"github_binary", "/usr/local/bin/wechat-mp-cli", "", "github-binary"},
		{"env_override_wins", "/home/u/node_modules/@fateforge/wechat-mp-cli-linux-x64/bin/wechat-mp-cli", "homebrew", "homebrew"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			updateBinaryExecutable = func() (string, error) { return tc.exe, nil }
			updateGetenv = func(k string) string {
				if k == "WECHAT_MP_CLI_INSTALL_METHOD" {
					return tc.env
				}
				return ""
			}
			if got := detectInstallMethod(); got != tc.want {
				t.Fatalf("detectInstallMethod() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestUpdateCheck_InstallMethodNpm proves Fix #3 surfaces through `update
// --check`: an npm-installed binary reports install_method=npm and a
// manager_command, not a hardcoded github-binary.
func TestUpdateCheck_InstallMethodNpm(t *testing.T) {
	restore := stubUpdateSeams(t)
	defer restore()
	updateBinaryExecutable = func() (string, error) {
		return "/home/u/node_modules/@fateforge/wechat-mp-cli-linux-x64/bin/wechat-mp-cli", nil
	}

	srv := newUpdateReleaseServer(t)
	defer srv.close()
	origAPI := updateBinaryGitHubAPI
	origClient := updateBinaryHTTPClient
	updateBinaryGitHubAPI = srv.api
	updateBinaryHTTPClient = srv.client
	defer func() { updateBinaryGitHubAPI = origAPI; updateBinaryHTTPClient = origClient }()

	env, exit := runUpdateCapture(t, "update", "--check")
	if exit != ExitOK {
		t.Fatalf("update --check exit = %d, want 0", exit)
	}
	data := envData(t, env)
	if data["install_method"] != "npm" {
		t.Fatalf("install_method = %v, want npm", data["install_method"])
	}
	if !strings.Contains(toStr(data["manager_command"]), "npm install -g @fateforge/wechat-mp-cli") {
		t.Fatalf("npm install should carry manager_command: %v", data["manager_command"])
	}
}

func toStr(v any) string {
	s, _ := v.(string)
	return s
}

// updateReleaseServer is a fake GitHub releases endpoint that serves a valid
// tar.gz archive, a matching checksums.txt, and a stub signature bundle. It
// pins the platform seam to linux/amd64 so the archive layout is deterministic
// regardless of the host OS; the signature verify itself is stubbed by the
// caller, but the SHA256 checksum is computed for real.
type updateReleaseServer struct {
	srv          *httptest.Server
	api          string
	client       *http.Client
	restorePlat  func()
	bundleStatus int // when non-zero, the bundle endpoint returns this status
}

func (s *updateReleaseServer) close() {
	s.srv.Close()
	s.restorePlat()
}

func newUpdateReleaseServer(t *testing.T) *updateReleaseServer {
	t.Helper()
	const tag = "v9.9.9"
	assetName := "wechat-mp-cli-9.9.9-linux-amd64.tar.gz"

	// Build the tar.gz archive containing the binary.
	archive := buildUpdateTarGz(t, "wechat-mp-cli", []byte("new-binary-bytes"))
	sum := sha256.Sum256(archive)
	checksums := hex.EncodeToString(sum[:]) + "  " + assetName + "\n"

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	rs := &updateReleaseServer{}

	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": tag,
				"html_url": srv.URL + "/release",
				"assets": []map[string]any{
					{"name": assetName, "browser_download_url": srv.URL + "/dl/" + assetName},
					{"name": "checksums.txt", "browser_download_url": srv.URL + "/dl/checksums.txt"},
					{"name": "checksums.txt.sigstore.json", "browser_download_url": srv.URL + "/dl/bundle.json"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/dl/"+assetName, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/dl/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, checksums)
	})
	mux.HandleFunc("/dl/bundle.json", func(w http.ResponseWriter, _ *http.Request) {
		if rs.bundleStatus != 0 {
			http.Error(w, "bundle unavailable", rs.bundleStatus)
			return
		}
		_, _ = io.WriteString(w, `{"bundle":"stub"}`)
	})

	origPlat := updateBinaryPlatform
	updateBinaryPlatform = func() (string, string) { return "linux", "amd64" }

	rs.srv = srv
	rs.api = srv.URL
	rs.client = srv.Client()
	rs.restorePlat = func() { updateBinaryPlatform = origPlat }
	return rs
}

// TestUpdate_NPMDrive: when the binary runs from node_modules (npm install),
// a bare `update` EXECUTES npm install -g (via the seam), then syncs the Skill,
// and reports status=updated with install_method=npm and binary_replaced=true.
// The seam ensures no real npm is called during testing.
func TestUpdate_NPMDrive(t *testing.T) {
	origExe := updateBinaryExecutable
	origPM := updateRunPackageManager
	origSync := updateSkillSync
	updateBinaryExecutable = func() (string, error) {
		return "/home/u/node_modules/@fateforge/wechat-mp-cli-linux-x64/bin/wechat-mp-cli", nil
	}
	pmCalled := false
	updateRunPackageManager = func(_ context.Context, method, _ string) error {
		pmCalled = true
		if method != "npm" {
			t.Errorf("package manager method = %q, want npm", method)
		}
		return nil
	}
	updateSkillSync = func(_ context.Context, _ string) error { return nil }
	defer func() {
		updateBinaryExecutable = origExe
		updateRunPackageManager = origPM
		updateSkillSync = origSync
	}()

	env, exit := runUpdateCapture(t, "update")
	if ok, _ := env["ok"].(bool); !ok {
		t.Fatalf("npm drive should be ok:true: %v", env)
	}
	if exit != ExitOK {
		t.Fatalf("npm drive exit = %d, want 0", exit)
	}
	if !pmCalled {
		t.Fatalf("updateRunPackageManager was not called for npm install")
	}
	data := envData(t, env)
	if data["install_method"] != "npm" {
		t.Fatalf("install_method = %v, want npm", data["install_method"])
	}
	if data["status"] != "updated" {
		t.Fatalf("status = %v, want updated", data["status"])
	}
	if data["skill_sync_status"] != "synced" {
		t.Fatalf("skill_sync_status = %v, want synced", data["skill_sync_status"])
	}
}

// TestUpdate_NPMDrive_Failure: when npm install -g fails, the envelope reports
// E_IO (exit 1, non-retryable), binary_replaced:false, and carries the command.
func TestUpdate_NPMDrive_Failure(t *testing.T) {
	origExe := updateBinaryExecutable
	origPM := updateRunPackageManager
	updateBinaryExecutable = func() (string, error) {
		return "/home/u/node_modules/@fateforge/wechat-mp-cli-linux-x64/bin/wechat-mp-cli", nil
	}
	updateRunPackageManager = func(_ context.Context, _, _ string) error {
		return errors.New("npm ERR! EACCES permission denied")
	}
	defer func() {
		updateBinaryExecutable = origExe
		updateRunPackageManager = origPM
	}()

	env, exit := runUpdateCapture(t, "update")
	if ok, _ := env["ok"].(bool); ok {
		t.Fatalf("npm failure should be ok:false: %v", env)
	}
	e := envError(t, env)
	if e["code"] != "E_IO" {
		t.Fatalf("npm failure code = %v, want E_IO", e["code"])
	}
	if e["retryable"] != false {
		t.Fatalf("npm failure must be non-retryable: %v", e)
	}
	if exit != ExitError {
		t.Fatalf("npm failure exit = %d, want 1", exit)
	}
	details, _ := e["details"].(map[string]any)
	if details["binary_replaced"] != false {
		t.Fatalf("npm failure binary_replaced must be false: %v", details)
	}
}

// TestUpdate_NPMDrive_DryRun: --dry-run for npm install previews the command
// and exits 0 without executing npm.
func TestUpdate_NPMDrive_DryRun(t *testing.T) {
	origExe := updateBinaryExecutable
	origPM := updateRunPackageManager
	updateBinaryExecutable = func() (string, error) {
		return "/home/u/node_modules/@fateforge/wechat-mp-cli-linux-x64/bin/wechat-mp-cli", nil
	}
	updateRunPackageManager = func(_ context.Context, _, _ string) error {
		t.Fatalf("npm must not be executed during --dry-run")
		return nil
	}
	defer func() {
		updateBinaryExecutable = origExe
		updateRunPackageManager = origPM
	}()

	env, exit := runUpdateCapture(t, "update", "--dry-run")
	if ok, _ := env["ok"].(bool); !ok {
		t.Fatalf("npm dry-run should be ok:true: %v", env)
	}
	if exit != ExitOK {
		t.Fatalf("npm dry-run exit = %d, want 0", exit)
	}
	data := envData(t, env)
	if !strings.Contains(toStr(data["command"]), "npm install -g @fateforge/wechat-mp-cli") {
		t.Fatalf("npm dry-run should carry npm command: %v", data["command"])
	}
}

func buildUpdateTarGz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{
		Name:     name,
		Mode:     0o755,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("tar write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}
