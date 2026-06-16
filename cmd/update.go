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
	rel, err := fetchBinaryRelease(cmd.Context(), updateTargetVersion)
	if err != nil {
		return fail(ExitRetryable, output.ErrNetwork, "checking release: "+err.Error(), true)
	}
	target := normalizeVersion(updateTargetVersion)
	if target == "" {
		target = normalizeVersion(rel.TagName)
	}
	skillCommand := updateSkillSyncCommand()

	available, versionKnown := false, false
	if cmp, ok := compareVersions(version, rel.TagName); ok {
		versionKnown = true
		available = cmp < 0 || (strings.TrimSpace(updateTargetVersion) != "" && target != normalizeVersion(version))
	}

	if updateCheck {
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
	if versionKnown && !available && strings.TrimSpace(updateTargetVersion) == "" {
		return printData(map[string]any{
			"current_version":  version,
			"latest_version":   rel.TagName,
			"update_available": false,
			"status":           "up_to_date",
		})
	}

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
