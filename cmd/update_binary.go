package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// This file gives wechat-mp-cli a self-contained binary self-update: download
// the platform archive + checksums.txt + Sigstore bundle, verify the signature
// in-process against the release-workflow identity, verify the archive SHA256,
// extract the binary, and replace the running executable. It does not depend on
// npm / go / pip being present on the user's machine.

const (
	updateBinaryName = "wechat-mp-cli"
	updateRepo       = "fatecannotbealtered/wechat-mp-cli"
	updateGitHubAPI  = "https://api.github.com"
	updateSkillRepo  = updateRepo
)

// integrityError marks a non-retryable supply-chain failure (missing/invalid
// signature, or checksum mismatch). Callers map it to E_INTEGRITY, never to a
// retryable network code.
type integrityError struct{ err error }

func (e *integrityError) Error() string { return e.err.Error() }
func (e *integrityError) Unwrap() error { return e.err }

func newIntegrityError(err error) error { return &integrityError{err: err} }

func isIntegrityError(err error) bool {
	var ie *integrityError
	return errors.As(err, &ie)
}

// replaceError marks a local failure during the replace stage (extract / file
// write / rename). permission distinguishes a permission denial (E_FORBIDDEN,
// exit 4) from a generic io/disk failure (E_IO, exit 1). These are local
// environment problems, never the retryable network class.
type replaceError struct {
	err        error
	permission bool
}

func (e *replaceError) Error() string { return e.err.Error() }
func (e *replaceError) Unwrap() error { return e.err }

func newReplaceError(err error) error {
	return &replaceError{err: err, permission: errors.Is(err, os.ErrPermission)}
}

func asReplaceError(err error) (*replaceError, bool) {
	var re *replaceError
	if errors.As(err, &re) {
		return re, true
	}
	return nil, false
}

// Testable seams.
var (
	updateBinaryHTTPClient = &http.Client{Timeout: 2 * time.Minute}
	updateBinaryGitHubAPI  = updateGitHubAPI
	updateBinaryPlatform   = func() (string, string) { return runtime.GOOS, runtime.GOARCH }
	updateBinaryExecutable = os.Executable
	updateBinaryApply      = applyUpdateBinary
	updateBinaryNow        = time.Now
	updateSkillSync        = runUpdateSkillSync
)

type updateReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type updateBinaryRelease struct {
	TagName string               `json:"tag_name"`
	HTMLURL string               `json:"html_url"`
	Assets  []updateReleaseAsset `json:"assets"`
}

type updateApplyResult struct {
	Status      string
	Path        string
	PendingPath string
}

// performBinaryUpdate downloads, verifies, and installs the target release
// binary. It returns the install status, the signature status (always
// "verified" on success), the installed path, the stage reached, and the error.
// The stage lets the caller build an honest failure envelope: everything up to
// and including verify_checksum touches only a temp dir (binary untouched);
// replace is the atomic commit point. Integrity failures are wrapped as
// non-retryable; replace-stage local failures are wrapped as replaceError so
// they are classified E_IO / E_FORBIDDEN, never the retryable network class.
func performBinaryUpdate(ctx context.Context, targetVersion string) (status, signatureStatus, resolvedVersion, stage string, err error) {
	stage = updateStageDiscover
	exe, err := updateBinaryExecutable()
	if err != nil {
		return "", "", "", stage, fmt.Errorf("resolving current executable: %w", err)
	}
	rel, err := fetchBinaryRelease(ctx, targetVersion)
	if err != nil {
		return "", "", "", stage, err
	}
	target := normalizeVersion(rel.TagName)
	if target == "" {
		return "", "", "", stage, errors.New("release is missing tag_name")
	}
	assetName, err := updateArchiveName(target)
	if err != nil {
		return "", "", target, stage, err
	}
	assetURL := findUpdateAssetURL(rel.Assets, assetName)
	if assetURL == "" {
		return "", "", target, stage, fmt.Errorf("release %s does not include asset %s", rel.TagName, assetName)
	}
	checksumURL := findUpdateAssetURL(rel.Assets, "checksums.txt")
	if checksumURL == "" {
		return "", "", target, stage, fmt.Errorf("release %s does not include checksums.txt", rel.TagName)
	}
	bundleURL := findUpdateAssetURL(rel.Assets, "checksums.txt.sigstore.json")

	tmpDir, err := os.MkdirTemp("", "wechat-mp-cli-update-*")
	if err != nil {
		return "", "", target, updateStageReplace, newReplaceError(fmt.Errorf("creating temp dir: %w", err))
	}
	// Always clean the temp dir, including on interrupt: a partial download must
	// never be trusted by a later run.
	defer func() { _ = os.RemoveAll(tmpDir) }()

	stage = updateStageDownload
	archivePath := filepath.Join(tmpDir, assetName)
	if err := downloadUpdateFile(ctx, assetURL, archivePath); err != nil {
		return "", "", target, stage, fmt.Errorf("downloading archive: %w", err)
	}
	checksumPath := filepath.Join(tmpDir, "checksums.txt")
	if err := downloadUpdateFile(ctx, checksumURL, checksumPath); err != nil {
		return "", "", target, stage, fmt.Errorf("downloading checksums: %w", err)
	}

	stage = updateStageVerifySignature
	signatureStatus, err = verifyUpdateChecksumSignature(ctx, checksumPath, bundleURL, tmpDir)
	if err != nil {
		return "", "", target, stage, newIntegrityError(fmt.Errorf("verifying release signature: %w", err))
	}
	stage = updateStageVerifyChecksum
	if err := verifyUpdateChecksum(archivePath, checksumPath, assetName); err != nil {
		return "", "", target, stage, newIntegrityError(fmt.Errorf("verifying archive: %w", err))
	}

	// Replace stage: extract + atomic swap. Local failures here (extract, file
	// write, rename, permission, disk) are NOT network failures — wrap them so
	// the caller emits E_IO / E_FORBIDDEN. The swap is atomic, so on failure the
	// installed binary is untouched (binary_replaced=false).
	stage = updateStageReplace
	binPath, err := extractUpdateArchive(archivePath, assetName, tmpDir)
	if err != nil {
		return "", "", target, stage, newReplaceError(fmt.Errorf("extracting archive: %w", err))
	}
	applied, err := updateBinaryApply(binPath, exe)
	if err != nil {
		return "", "", target, stage, newReplaceError(fmt.Errorf("installing update: %w", err))
	}
	return applied.Status, signatureStatus, target, stage, nil
}

// Update stage names, shared between the binary update flow and the command
// handler so the failure envelope's `stage` cannot drift.
const (
	updateStageDiscover        = "discover"
	updateStageDownload        = "download"
	updateStageVerifySignature = "verify_signature"
	updateStageVerifyChecksum  = "verify_checksum"
	updateStageReplace         = "replace"
	updateStageSkillSync       = "skill_sync"
)

// verifyUpdateChecksumSignature enforces a mandatory, in-process Sigstore
// signature check on checksums.txt. There is no skip path.
func verifyUpdateChecksumSignature(ctx context.Context, checksumPath, bundleURL, tmpDir string) (string, error) {
	if strings.TrimSpace(bundleURL) == "" {
		return "missing", errors.New("release does not include checksums.txt.sigstore.json; refusing to install an unsigned release")
	}
	bundlePath := filepath.Join(tmpDir, "checksums.txt.sigstore.json")
	if err := downloadUpdateFile(ctx, bundleURL, bundlePath); err != nil {
		return "download_failed", fmt.Errorf("downloading checksum signature bundle: %w", err)
	}
	if err := updateVerifySignature(checksumPath, bundlePath, updateSignerIdentityRegexp()); err != nil {
		return "failed", err
	}
	return "verified", nil
}

func fetchBinaryRelease(ctx context.Context, targetVersion string) (*updateBinaryRelease, error) {
	base := strings.TrimRight(updateBinaryGitHubAPI, "/")
	url := base + "/repos/" + updateRepo + "/releases/latest"
	if v := normalizeVersion(targetVersion); v != "" {
		url = base + "/repos/" + updateRepo + "/releases/tags/" + canonicalVersionTag(v)
	}
	req, err := newUpdateRequest(ctx, url, "application/json")
	if err != nil {
		return nil, err
	}
	resp, err := updateBinaryHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s returned %d", url, resp.StatusCode)
	}
	var rel updateBinaryRelease
	if err := json.Unmarshal(data, &rel); err != nil {
		return nil, fmt.Errorf("parsing release JSON: %w", err)
	}
	return &rel, nil
}

func updateArchiveName(ver string) (string, error) {
	goos, goarch := updateBinaryPlatform()
	platform, ok := map[string]string{"darwin": "darwin", "linux": "linux", "windows": "windows"}[goos]
	if !ok {
		return "", fmt.Errorf("unsupported update platform: %s-%s", goos, goarch)
	}
	arch, ok := map[string]string{"amd64": "amd64", "arm64": "arm64"}[goarch]
	if goos == "windows" && goarch == "arm64" {
		arch, ok = "amd64", true
	}
	if !ok {
		return "", fmt.Errorf("unsupported update platform: %s-%s", goos, goarch)
	}
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("%s-%s-%s-%s%s", updateBinaryName, normalizeVersion(ver), platform, arch, ext), nil
}

func findUpdateAssetURL(assets []updateReleaseAsset, name string) string {
	for _, a := range assets {
		if a.Name == name {
			return a.BrowserDownloadURL
		}
	}
	return ""
}

func newUpdateRequest(ctx context.Context, url, accept string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", updateBinaryName)
	if tok := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	return req, nil
}

func downloadUpdateFile(ctx context.Context, url, dest string) error {
	req, err := newUpdateRequest(ctx, url, "application/octet-stream")
	if err != nil {
		return err
	}
	resp, err := updateBinaryHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GET %s returned %d", url, resp.StatusCode)
	}
	tmp := dest + ".part"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func verifyUpdateChecksum(archivePath, checksumPath, assetName string) error {
	checksumData, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("reading checksums: %w", err)
	}
	expected := ""
	for _, line := range strings.Split(string(checksumData), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if filepath.Base(fields[len(fields)-1]) == assetName {
			expected = strings.ToLower(fields[0])
			break
		}
	}
	if expected == "" {
		return fmt.Errorf("checksum for %s not found", assetName)
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("reading archive: %w", err)
	}
	defer func() { _ = f.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return fmt.Errorf("hashing archive: %w", err)
	}
	if hex.EncodeToString(hash.Sum(nil)) != expected {
		return fmt.Errorf("checksum mismatch for %s", assetName)
	}
	return nil
}

func extractUpdateArchive(archivePath, assetName, tmpDir string) (string, error) {
	if strings.HasSuffix(assetName, ".zip") {
		return extractUpdateZip(archivePath, tmpDir)
	}
	if strings.HasSuffix(assetName, ".tar.gz") {
		return extractUpdateTarGz(archivePath, tmpDir)
	}
	return "", fmt.Errorf("unsupported archive type: %s", assetName)
}

func extractUpdateZip(archivePath, tmpDir string) (string, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = zr.Close() }()
	want := updateArchiveBinaryName()
	for _, f := range zr.File {
		if filepath.Base(f.Name) != want {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer func() { _ = rc.Close() }()
		return writeExtractedUpdateBinary(tmpDir, want, rc)
	}
	return "", fmt.Errorf("%s not found in archive", want)
}

func extractUpdateTarGz(archivePath, tmpDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	want := updateArchiveBinaryName()
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != want {
			continue
		}
		return writeExtractedUpdateBinary(tmpDir, want, tr)
	}
	return "", fmt.Errorf("%s not found in archive", want)
}

func updateArchiveBinaryName() string {
	goos, _ := updateBinaryPlatform()
	if goos == "windows" {
		return updateBinaryName + ".exe"
	}
	return updateBinaryName
}

func writeExtractedUpdateBinary(tmpDir, name string, r io.Reader) (string, error) {
	outDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	outPath := filepath.Join(outDir, name)
	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return outPath, nil
}

func applyUpdateBinary(src, dst string) (updateApplyResult, error) {
	if runtime.GOOS == "windows" {
		return scheduleWindowsBinaryReplace(src, dst)
	}
	mode := os.FileMode(0o755)
	if st, err := os.Stat(dst); err == nil {
		mode = st.Mode().Perm()
		if mode&0o111 == 0 {
			mode |= 0o755
		}
	}
	tmpName := fmt.Sprintf(".%s.update-%d", filepath.Base(dst), updateBinaryNow().UnixNano())
	tmpPath := filepath.Join(filepath.Dir(dst), tmpName)
	if err := updateCopyFile(src, tmpPath, mode); err != nil {
		return updateApplyResult{}, err
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		_ = os.Remove(tmpPath)
		return updateApplyResult{}, err
	}
	return updateApplyResult{Status: "installed", Path: dst}, nil
}

func scheduleWindowsBinaryReplace(src, dst string) (updateApplyResult, error) {
	pending := dst + ".new"
	if err := updateCopyFile(src, pending, 0o755); err != nil {
		return updateApplyResult{}, err
	}
	scriptPath := filepath.Join(os.TempDir(), fmt.Sprintf("wechat-mp-cli-update-%d.cmd", updateBinaryNow().UnixNano()))
	script := strings.Join([]string{
		"@echo off",
		"setlocal",
		"set \"PENDING=" + batchEscape(pending) + "\"",
		"set \"TARGET=" + batchEscape(dst) + "\"",
		"for /L %%I in (1,1,30) do (",
		"  move /Y \"%PENDING%\" \"%TARGET%\" > nul 2>&1",
		"  if not exist \"%PENDING%\" goto done",
		"  ping 127.0.0.1 -n 2 > nul",
		")",
		":done",
		"del \"%~f0\" > nul 2>&1",
		"",
	}, "\r\n")
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		return updateApplyResult{}, err
	}
	if err := exec.Command("cmd", "/C", "start", "", "/B", scriptPath).Start(); err != nil {
		return updateApplyResult{}, err
	}
	return updateApplyResult{Status: "scheduled", Path: dst, PendingPath: pending}, nil
}

func batchEscape(s string) string {
	return strings.NewReplacer("%", "%%", "^", "^^", "&", "^&", "<", "^<", ">", "^>", "|", "^|").Replace(s)
}

func updateCopyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}

func updateSkillSyncCommand() string {
	return "npx skills add " + updateSkillRepo + " -y -g"
}

func runUpdateSkillSync(ctx context.Context, repo string) error {
	cmd := exec.CommandContext(ctx, "npx", "skills", "add", repo, "-y", "-g")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, truncate(msg, 300))
		}
		return err
	}
	return nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// --- version helpers ---

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "refs/tags/")
	v = strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V")
	return v
}

func canonicalVersionTag(v string) string {
	v = normalizeVersion(v)
	if v == "" || v == "latest" {
		return "latest"
	}
	return "v" + v
}

func parseVersion(v string) ([3]int, bool) {
	var out [3]int
	v = normalizeVersion(v)
	if v == "" || v == "dev" || v == "(devel)" || v == "latest" {
		return out, false
	}
	if idx := strings.IndexAny(v, "-+"); idx >= 0 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

func compareVersions(current, latest string) (int, bool) {
	cur, ok := parseVersion(current)
	if !ok {
		return 0, false
	}
	newest, ok := parseVersion(latest)
	if !ok {
		return 0, false
	}
	for i := range cur {
		if cur[i] < newest[i] {
			return -1, true
		}
		if cur[i] > newest[i] {
			return 1, true
		}
	}
	return 0, true
}
