package cmd

import (
	"runtime"
	"time"

	"github.com/fatecannotbealtered/wechat-mp-cli/internal/config"
	"github.com/spf13/cobra"
)

var contextCmd = readCommand(&cobra.Command{
	Use:   "context",
	Short: "Print runtime context for AI agents",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return handleError(err)
		}
		return printData(map[string]any{
			"tool":           "wechat-mp-cli",
			"version":        version,
			"schema_version": "1.0",
			"risk_tier":      toolRiskTier,
			"blast_radius":   toolBlastRadius,
			"go": map[string]any{
				"version": runtime.Version(),
				"os":      runtime.GOOS,
				"arch":    runtime.GOARCH,
			},
			"credentials": contextCredentials(cfg),
			"config": map[string]any{
				"path":            config.FilePath(),
				"api_base":        cfg.APIBase,
				"api_proxy":       cfg.APIProxy,
				"api_proxy_set":   cfg.APIProxy != "",
				"default_account": cfg.DefaultAccount,
				"accounts":        config.PublicAccounts(cfg),
			},
			"next_steps": []string{
				"Run wechat-mp-cli doctor --compact before write commands.",
				"Run wechat-mp-cli reference --compact for the live command contract.",
				"For writes, run --dry-run first and repeat with --confirm <confirm_token>.",
			},
		})
	},
}, "context")

// contextCredentials reports CLI-SPEC §15.1 lifecycle state for the default
// account: validity and expiry of the cached token, always redacted.
func contextCredentials(cfg *config.Config) map[string]any {
	account, err := config.ResolveAccount(cfg, "")
	if err != nil {
		return map[string]any{"configured": false}
	}
	out := map[string]any{
		"configured":  account.AppID != "" && account.AppSecret != "",
		"account":     account.Alias,
		"refreshable": account.AppSecret != "",
		"storage":     account.Storage,
		"valid":       false,
	}
	if cached, ok := config.LoadCachedToken(account.AppID); ok {
		out["valid"] = time.Until(cached.ExpiresAt) > 0
		out["expires_at"] = cached.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return out
}

func init() {
	rootCmd.AddCommand(contextCmd)
}
