package cmd

import (
	"strings"

	"github.com/fatecannotbealtered/wechat-mp-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	updateCheck         bool
	updateTargetVersion string
)

type updateConfirmPayload struct {
	TargetVersion    string `json:"target_version"`
	SkillSyncCommand string `json:"skill_sync_command"`
}

var updateCmd = writeCommand(&cobra.Command{
	Use:   "update",
	Short: "Update wechat-mp-cli to a verified GitHub release and sync the Skill",
	Long: `Download the matching GitHub Release binary, verify the Sigstore signature on
checksums.txt in-process against this repo's tagged release workflow identity,
verify the archive SHA256, replace the running binary, and sync the Skill
directory. An unsigned or unverifiable release is refused; there is no skip path
and no dependency on npm/go/pip being installed.

Use --check to inspect availability. Writes require --dry-run first, then
--confirm <confirm_token>.`,
	Args: cobra.NoArgs,
	RunE: runUpdate,
}, "medium", "replaces the local wechat-mp-cli binary and syncs the Skill directory")

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
		rel, err := fetchBinaryRelease(cmd.Context(), updateTargetVersion)
		if err != nil {
			return printData(map[string]any{
				"current_version":    version,
				"update_available":   false,
				"install_method":     "github-binary",
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
		return printData(map[string]any{
			"current_version":    version,
			"latest_version":     rel.TagName,
			"target_version":     target,
			"update_available":   available,
			"release_url":        rel.HTMLURL,
			"install_method":     "github-binary",
			"signature_status":   "not_checked",
			"skill_sync_command": skillCommand,
		})
	}

	// Write path: the release is fetched and verified only at --confirm execution
	// (performBinaryUpdate). The dry-run preview and confirm-token binding use the
	// requested target (empty = latest), so dry-run is offline and a bare write
	// returns E_CONFIRMATION_REQUIRED before any network call.
	target := normalizeVersion(updateTargetVersion)
	payload := updateConfirmPayload{TargetVersion: target, SkillSyncCommand: skillCommand}
	preview := map[string]any{
		"action": "update wechat-mp-cli",
		"changes": []map[string]any{
			{"operation": "download, verify signature + checksum, replace binary", "target_version": target},
			{"operation": "sync skill directory", "command": skillCommand},
		},
	}
	proceed, err := confirmWrite("update wechat-mp-cli", payload, preview)
	if !proceed {
		return err
	}

	// Download + verify (in-process Sigstore) + checksum + replace binary.
	status, sigStatus, _, err := performBinaryUpdate(cmd.Context(), target)
	if err != nil {
		if isIntegrityError(err) {
			// Non-retryable: a missing/invalid signature or checksum mismatch is a
			// supply-chain red flag, not a transient blip.
			return fail(ExitError, output.ErrIntegrity, err.Error(), false)
		}
		return fail(ExitRetryable, output.ErrNetwork, "update failed: "+err.Error(), true)
	}
	if err := updateSkillSync(cmd.Context(), updateSkillRepo); err != nil {
		return fail(ExitRetryable, output.ErrNetwork, "syncing skill directory: "+err.Error(), true)
	}

	resultStatus := "updated"
	if status == "scheduled" {
		resultStatus = "scheduled"
	}
	return printData(map[string]any{
		"status":             resultStatus,
		"previous_version":   version,
		"current_version":    target,
		"signature_status":   sigStatus,
		"signature_verified": sigStatus == "verified",
		"skill_sync_status":  "synced",
		"skill_sync_command": skillCommand,
		"next_step":          "run \"wechat-mp-cli changelog --since " + version + "\" to see what changed",
	})
}
