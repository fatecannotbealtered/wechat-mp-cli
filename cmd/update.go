package cmd

import (
	"context"
	"strings"
	"time"

	"github.com/fatecannotbealtered/wechat-mp-cli/internal/config"
	"github.com/fatecannotbealtered/wechat-mp-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	updateCheck         bool
	updateTargetVersion string
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update wechat-mp-cli to a verified GitHub release and sync the Skill",
	Long: `Download the matching GitHub Release binary, verify the Sigstore signature on
checksums.txt in-process against this repo's tagged release workflow identity,
verify the archive SHA256, replace the running binary, and sync the Skill
directory. An unsigned or unverifiable release is refused; there is no skip path
and no dependency on npm/go/pip being installed.

A bare 'update' performs the whole update in one call — self-update is a single,
self-verifying operation and takes no confirm token. Use --check for a read-only
probe and --dry-run for a read-only preview of the plan; neither changes
anything.`,
	Args: cobra.NoArgs,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&updateCheck, "check", false, "Check whether a newer version is available without installing")
	updateCmd.Flags().StringVar(&updateTargetVersion, "target-version", "", "Install a specific version (for example 1.2.3 or v1.2.3)")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, _ []string) error {
	skillCommand := updateSkillSyncCommand()

	// --check is a best-effort read: a network / rate-limit failure must not be a
	// hard error, otherwise an offline check looks like a broken tool.
	if updateCheck {
		return runUpdateCheck(cmd.Context(), skillCommand)
	}

	target := normalizeVersion(updateTargetVersion)

	// --dry-run is a read-only preview of the plan. It is no longer a gate: it
	// issues NO confirm_token and NO expires_at, and is never required before a
	// bare `update`.
	if dryRun {
		return printData(map[string]any{
			"action":          "update wechat-mp-cli",
			"current_version": version,
			"target_version":  target,
			"changes": []map[string]any{
				{"operation": "download, verify signature + checksum, replace binary", "target_version": target},
				{"operation": "sync skill directory", "command": skillCommand},
			},
			"skill_sync_command": skillCommand,
		})
	}

	// Single command, no confirm token: resolve -> verify integrity -> replace
	// binary -> sync Skill, all in one call. Self-update is exempt from the §7
	// write gate; the safety guarantee is the in-process signature verification.
	ctx := cmd.Context()

	// Idempotent: when already on the resolved (latest or requested) version,
	// return ok with a no-op result so an agent may call update freely.
	if rel, derr := fetchBinaryRelease(ctx, target); derr == nil {
		resolved := target
		if resolved == "" {
			resolved = normalizeVersion(rel.TagName)
		}
		if cmp, ok := compareVersions(version, resolved); ok && cmp >= 0 {
			return printData(map[string]any{
				"status":             "noop",
				"previous_version":   version,
				"current_version":    version,
				"binary_replaced":    false,
				"update_available":   false,
				"skill_sync_status":  "skipped",
				"skill_sync_command": skillCommand,
			})
		}
	}

	_, sigStatus, resolved, stage, err := performBinaryUpdate(ctx, target)
	if err != nil {
		return reportUpdateFailure(ctx, stage, err, skillCommand)
	}

	// Binary is now on the new version. Skill sync runs AFTER the swap and is
	// independently replayable; a failure here is PARTIAL SUCCESS, not a hard
	// error, so the agent knows it is on the new binary.
	if err := updateSkillSync(ctx, updateSkillRepo); err != nil {
		if ctx.Err() != nil {
			return reportUpdateInterrupted(updateStageSkillSync, resolved, true, skillCommand)
		}
		return failWithDetails(ExitError, output.ErrNetwork,
			"binary updated to "+resolved+" but skill sync failed: "+err.Error(),
			map[string]any{
				"stage":              updateStageSkillSync,
				"current_version":    resolved,
				"binary_replaced":    true,
				"skill_sync_status":  "failed",
				"skill_sync_command": skillCommand,
				"next_step":          "run \"" + skillCommand + "\", then \"wechat-mp-cli changelog --since " + version + "\"",
			}, true)
	}

	return printData(map[string]any{
		"status":             "updated",
		"previous_version":   version,
		"current_version":    resolved,
		"binary_replaced":    true,
		"signature_status":   sigStatus,
		"signature_verified": sigStatus == "verified",
		"skill_sync_status":  "synced",
		"skill_sync_command": skillCommand,
		"next_step":          "run \"wechat-mp-cli changelog --since " + version + "\" to see what changed",
	})
}

func runUpdateCheck(ctx context.Context, skillCommand string) error {
	installMethod := detectInstallMethod()
	rel, err := fetchBinaryRelease(ctx, updateTargetVersion)
	if err != nil {
		return printData(map[string]any{
			"current_version":    version,
			"update_available":   false,
			"install_method":     installMethod,
			"signature_status":   "not_checked",
			"skill_sync_command": skillCommand,
			"error":              "could not reach GitHub releases: " + err.Error(),
		})
	}
	target := normalizeVersion(updateTargetVersion)
	if target == "" {
		target = normalizeVersion(rel.TagName)
	}
	available := false
	if cmp, ok := compareVersions(version, rel.TagName); ok {
		available = cmp < 0 || (strings.TrimSpace(updateTargetVersion) != "" && target != normalizeVersion(version))
	}
	data := map[string]any{
		"current_version":    version,
		"latest_version":     rel.TagName,
		"target_version":     target,
		"update_available":   available,
		"release_url":        rel.HTMLURL,
		"install_method":     installMethod,
		"signature_status":   "not_checked",
		"skill_sync_command": skillCommand,
	}
	// When the binary is owned by a package manager, point the agent at the
	// channel that owns it instead of the in-place self-updater.
	if mgr := updateManagerCommand(installMethod); mgr != "" {
		data["manager_command"] = mgr
	}
	// Refresh the local notice cache so future commands can piggyback it onto
	// meta.notices. The severity is graded here (at check time) and stored, so
	// the cached read carries the right level.
	refreshUpdateNoticeCache(rel.TagName, rel.HTMLURL)
	if notice := updateNoticesFromRelease(version, rel.TagName, rel.HTMLURL,
		nowRFC3339(), updateRecommendedCommand(), installMethod); notice != nil {
		data["notices"] = []any{notice}
	}
	return printData(data)
}

// refreshUpdateNoticeCache rebuilds the local update-notice cache from the
// resolved latest release: it stores the graded notice when an update is
// available and clears the cache otherwise, so an already-current tool never
// leaves a stale notice behind. Cache write failures are non-fatal — a check
// that cannot persist the notice still returns its result.
func refreshUpdateNoticeCache(latestTag, releaseURL string) {
	notice := updateNoticesFromRelease(version, latestTag, releaseURL,
		nowRFC3339(), updateRecommendedCommand(), detectInstallMethod())
	if notice == nil {
		_ = config.ClearCachedUpdateNotice()
		return
	}
	_ = config.SaveCachedUpdateNotice(notice)
}

// updateRecommendedCommand is the single command an agent runs to self-update.
func updateRecommendedCommand() string { return "wechat-mp-cli update" }

func nowRFC3339() string { return updateBinaryNow().UTC().Format(time.RFC3339) }

// reportUpdateFailure builds the staged failure envelope. Everything before the
// binary swap leaves the installed binary untouched, so the post-state is always
// "still on the running version", binary_replaced=false. The failure is
// classified by the agent's next action, not the raw cause.
func reportUpdateFailure(ctx context.Context, stage string, err error, skillCommand string) error {
	// An interrupt (SIGINT/SIGTERM cancels the context) takes precedence: the
	// real cause is the cancellation, not a downstream symptom.
	if ctx.Err() != nil {
		return reportUpdateInterrupted(stage, version, false, skillCommand)
	}

	details := map[string]any{
		"stage":              stage,
		"current_version":    version,
		"binary_replaced":    false,
		"skill_sync_status":  "skipped",
		"skill_sync_command": skillCommand,
	}

	// Integrity failure: fail closed, non-retryable. A forged or corrupt release
	// is not a transient blip to loop on.
	if isIntegrityError(err) {
		return failWithDetails(ExitError, output.ErrIntegrity, err.Error(), details, false)
	}

	// Replace-stage local failure: permission -> E_FORBIDDEN (exit 4); other
	// io/disk -> E_IO (exit 1). Never the retryable network class.
	if re, ok := asReplaceError(err); ok {
		if re.permission {
			return failWithDetails(ExitAuth, output.ErrForbidden,
				"update failed during replace: "+err.Error(), details, false)
		}
		return failWithDetails(ExitError, output.ErrIO,
			"update failed during replace: "+err.Error(), details, false)
	}

	// discover / download: classify the HTTP failure by status rather than
	// collapsing every non-2xx into E_NETWORK. 404 -> E_NOT_FOUND (a requested
	// --target-version that does not exist is not retryable), 429 ->
	// E_RATE_LIMITED, 5xx -> E_SERVER, timeout -> E_TIMEOUT; transport failures
	// stay E_NETWORK. All but 404 are retryable, and re-running `update` is
	// idempotent.
	code, exit, retryable := classifyUpdateNetworkError(err)
	return failWithDetails(exit, code, "update failed: "+err.Error(), details, retryable)
}

// reportUpdateInterrupted emits the terminal JSON envelope after a SIGINT/SIGTERM
// so an interrupted agent always receives a parseable terminal state. The
// message states the version the tool is actually running now per the stage
// invariant: before the swap -> no change; after the swap during skill_sync ->
// partial success on the new binary.
func reportUpdateInterrupted(stage, currentVersion string, binaryReplaced bool, skillCommand string) error {
	details := map[string]any{
		"stage":              stage,
		"current_version":    currentVersion,
		"binary_replaced":    binaryReplaced,
		"skill_sync_command": skillCommand,
	}
	var msg string
	if binaryReplaced {
		details["skill_sync_status"] = "failed"
		msg = "update interrupted after binary replace: now on " + currentVersion +
			"; run \"" + skillCommand + "\", then \"wechat-mp-cli changelog --since " + version + "\""
	} else {
		details["skill_sync_status"] = "skipped"
		msg = "update cancelled, no change, still on " + currentVersion + " — re-run \"wechat-mp-cli update\", it is idempotent"
	}
	return failWithDetails(ExitInterrupted, output.ErrInterrupted, msg, details, true)
}
