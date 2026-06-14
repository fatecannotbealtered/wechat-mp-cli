package e2e

import (
	"strings"
	"testing"
)

// TestBatch_InfoBatchAggregatesPerItem drives the class-A batch read end to end:
// one envelope, an items[] keyed by the input openid, and a total/succeeded/
// failed summary (CLI-SPEC §15.5). The mock returns profiles for OPENID and
// OPENID2 only, so a third, un-returned openid must surface as a per-item
// failure without failing the whole command.
func TestBatch_InfoBatchAggregatesPerItem(t *testing.T) {
	r := runCLI(t.TempDir(), nil, "--format", "json",
		"user", "info-batch", "--openids", "OPENID,OPENID2,GHOST")
	data := decodeOK(t, r)

	items, _ := data["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("want 3 items (input order preserved), got %d: %s", len(items), r.Stdout)
	}
	// Input order is preserved so the agent can zip results back to inputs.
	wantTargets := []string{"OPENID", "OPENID2", "GHOST"}
	for i, raw := range items {
		it := raw.(map[string]any)
		if it["target"] != wantTargets[i] {
			t.Fatalf("item[%d].target = %v, want %s", i, it["target"], wantTargets[i])
		}
	}
	ghost := items[2].(map[string]any)
	if ghost["ok"] != false {
		t.Fatalf("un-returned openid should be ok=false: %s", r.Stdout)
	}
	gErr, _ := ghost["error"].(map[string]any)
	if gErr == nil || gErr["code"] != "E_NOT_FOUND" {
		t.Fatalf("ghost item should carry E_NOT_FOUND error: %s", r.Stdout)
	}

	summary, _ := data["summary"].(map[string]any)
	if summary["total"] != float64(3) || summary["succeeded"] != float64(2) || summary["failed"] != float64(1) {
		t.Fatalf("summary mismatch: %v", summary)
	}
}

// TestBatch_InfoBatchDeDupesAndTagsUntrusted: duplicate inputs collapse to one
// target, and the items subtree (carrying WeChat-controlled nickname/remark) is
// declared _untrusted (§15.8).
func TestBatch_InfoBatchDeDupesAndTagsUntrusted(t *testing.T) {
	r := runCLI(t.TempDir(), nil, "--format", "json",
		"user", "info-batch", "--openids", "OPENID", "--openids", "OPENID")
	data := decodeOK(t, r)
	items, _ := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("duplicate openids should de-dupe to 1 item, got %d: %s", len(items), r.Stdout)
	}
	tagged, _ := data["_untrusted"].([]any)
	if len(tagged) == 0 || tagged[0] != "items" {
		t.Fatalf("items subtree should be tagged _untrusted, got: %s", r.Stdout)
	}
}

// TestBatch_InfoBatchEmptyIsValidationError: an empty target list is a usage
// error, never a silent no-op (§15.1).
func TestBatch_InfoBatchEmptyIsValidationError(t *testing.T) {
	r := runCLI(t.TempDir(), nil, "--format", "json", "user", "info-batch")
	if r.ExitCode != 2 {
		t.Fatalf("exit code = %d, want 2\nstdout: %s", r.ExitCode, r.Stdout)
	}
}

// TestBatch_MassSendDangerousGate: a critical mass-send dry-run without
// --dangerous is rejected with exit 5 even though it would otherwise mint a
// token (§15.4).
func TestBatch_MassSendDangerousGate(t *testing.T) {
	r := runCLI(t.TempDir(), nil, "--format", "json",
		"message", "mass", "send", "--openids", "OPENID", "--mpnews-media-id", "MEDIA_ID", "--dry-run")
	if r.ExitCode != 5 {
		t.Fatalf("exit code = %d, want 5\nstdout: %s", r.ExitCode, r.Stdout)
	}
	env := decodeEnvelope(t, r.Stdout)
	if env.OK || env.Error == nil || !strings.Contains(env.Error.Message, "--dangerous") {
		t.Fatalf("want dangerous-gate error, got: %s", r.Stdout)
	}
}

// TestBatch_MassSendDryRunPreviewsBlastRadius: the dry-run preview states the
// action, total, and full resolved target set so the blast radius is auditable
// before confirming (§15.2).
func TestBatch_MassSendDryRunPreviewsBlastRadius(t *testing.T) {
	r := runCLI(t.TempDir(), nil, "--format", "json",
		"message", "mass", "send", "--openids", "OPENID,OPENID2", "--mpnews-media-id", "MEDIA_ID",
		"--dangerous", "--dry-run")
	data := decodeOK(t, r)
	if strings.TrimSpace(data["confirm_token"].(string)) == "" {
		t.Fatalf("dry-run should return a confirm_token: %s", r.Stdout)
	}
	preview, _ := data["preview"].(map[string]any)
	if preview == nil || preview["total"] != float64(2) {
		t.Fatalf("preview should report total=2: %s", r.Stdout)
	}
	targets, _ := preview["targets"].([]any)
	if len(targets) != 2 {
		t.Fatalf("preview should list the full target set: %s", r.Stdout)
	}
}

// TestBatch_MassSendConfirmThenReplayConflict: one confirm token authorizes the
// whole batch once; a replay is rejected with E_CONFLICT (§15.3, single-use).
func TestBatch_MassSendConfirmThenReplayConflict(t *testing.T) {
	home := t.TempDir()
	args := []string{"--format", "json", "message", "mass", "send",
		"--openids", "OPENID,OPENID2", "--mpnews-media-id", "MEDIA_ID", "--dangerous"}

	dry := runCLI(home, nil, append(append([]string{}, args...), "--dry-run")...)
	data := decodeOK(t, dry)
	token := strings.TrimSpace(data["confirm_token"].(string))
	if token == "" {
		t.Fatalf("no confirm_token: %s", dry.Stdout)
	}

	run := runCLI(home, nil, append(append([]string{}, args...), "--confirm", token)...)
	decodeOK(t, run)

	// Replay the consumed token: must be rejected, not re-broadcast.
	replay := runCLI(home, nil, append(append([]string{}, args...), "--confirm", token)...)
	if replay.ExitCode != 6 {
		t.Fatalf("replay exit code = %d, want 6 (E_CONFLICT)\nstdout: %s", replay.ExitCode, replay.Stdout)
	}
	env := decodeEnvelope(t, replay.Stdout)
	if env.OK || env.Error == nil || env.Error.Code != "E_CONFLICT" {
		t.Fatalf("replay should be E_CONFLICT, got: %s", replay.Stdout)
	}
}
