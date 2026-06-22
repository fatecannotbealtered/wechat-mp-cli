package cmd

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/fatecannotbealtered/wechat-mp-cli/internal/config"
	"github.com/spf13/cobra"
)

type doctorCheck struct {
	Name    string `json:"check"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
}

var doctorCmd = readCommand(&cobra.Command{
	Use:   "doctor",
	Short: "Check local configuration and WeChat API readiness",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return handleError(err)
		}
		checks := []doctorCheck{
			checkConfigDir(),
			checkAccountConfig(cfg),
			checkDefaultAccount(cfg),
			checkCredentials(cfg),
			checkAPIProxy(cfg),
			{
				Name:    "release_readiness",
				Status:  releaseReadinessCheckStatus(),
				Message: "release level: " + buildReleaseReadiness().Level,
				Fix:     releaseReadinessCheckFix(),
			},
		}
		blocking := false
		for _, check := range checks {
			if check.Status == "fail" {
				blocking = true
			}
		}
		notices := []map[string]any{}
		if len(cfg.Accounts) == 0 {
			notices = append(notices, map[string]any{
				"type":    "setup_hint",
				"message": "Add an account with setup account add, or set WECHAT_MP_CLI_APP_ID and WECHAT_MP_CLI_APP_SECRET.",
			})
		}
		// doctor MAY actively check for an update with a short timeout (CLI-SPEC
		// §14); a network failure must not make doctor fail by itself. The graded
		// notice is refreshed into the local cache and surfaced in data.notices
		// as the fresh/active view.
		if notice := doctorUpdateNotice(cmd.Context()); notice != nil {
			notices = append(notices, notice)
		}
		return printData(map[string]any{
			"ok":          !blocking,
			"config_path": config.FilePath(),
			"checks":      checks,
			"notices":     notices,
		})
	},
}, "doctor")

func init() {
	rootCmd.AddCommand(doctorCmd)
}

// doctorUpdateNotice does a best-effort, short-timeout active update check. It
// refreshes the local notice cache and returns the graded notice when an update
// is available; any network/timeout failure (or already-current) yields nil so
// doctor never fails on it. The cache write lets later commands piggyback the
// same notice onto meta.notices.
func doctorUpdateNotice(ctx context.Context) map[string]any {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	rel, err := fetchBinaryRelease(ctx, "")
	if err != nil {
		return nil
	}
	refreshUpdateNoticeCache(rel.TagName, rel.HTMLURL)
	return updateNoticesFromRelease(version, rel.TagName, rel.HTMLURL,
		nowRFC3339(), updateRecommendedCommand(), "github-binary")
}

func checkConfigDir() doctorCheck {
	dir := config.Dir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return doctorCheck{Name: "config_dir", Status: "fail", Message: err.Error()}
	}
	if info, err := os.Stat(dir); err != nil {
		return doctorCheck{Name: "config_dir", Status: "fail", Message: err.Error()}
	} else if !info.IsDir() {
		return doctorCheck{Name: "config_dir", Status: "fail", Message: filepath.Clean(dir) + " is not a directory"}
	}
	return doctorCheck{Name: "config_dir", Status: "pass", Message: filepath.Clean(dir)}
}

// checkCredentials reports the CLI-SPEC §15.1 token lifecycle state: cached
// token validity and expiry, with an actionable renewal hint near expiry.
func checkCredentials(cfg *config.Config) doctorCheck {
	account, err := config.ResolveAccount(cfg, "")
	if err != nil {
		return doctorCheck{
			Name: "credentials", Status: "warn",
			Message: "no resolvable default account: " + err.Error(),
			Fix:     "run 'wechat-mp-cli setup account add' or set WECHAT_MP_CLI_APP_ID and WECHAT_MP_CLI_APP_SECRET",
		}
	}
	if account.AppSecret == "" {
		return doctorCheck{
			Name: "credentials", Status: "fail",
			Message: "account " + account.Alias + " has no app_secret; tokens cannot be refreshed",
			Fix:     "re-run 'wechat-mp-cli setup account add' with the app secret",
		}
	}
	cached, ok := config.LoadCachedToken(account.AppID)
	if !ok || time.Until(cached.ExpiresAt) <= 0 {
		return doctorCheck{
			Name: "credentials", Status: "pass",
			Message: "no valid cached token; one will be fetched via stable_token on the next API call",
		}
	}
	remaining := time.Until(cached.ExpiresAt)
	if remaining < 15*time.Minute {
		return doctorCheck{
			Name: "credentials", Status: "warn",
			Message: "cached token expires at " + cached.ExpiresAt.UTC().Format(time.RFC3339),
			Fix:     "run 'wechat-mp-cli token refresh' to renew before long operations",
		}
	}
	return doctorCheck{
		Name: "credentials", Status: "pass",
		Message: "cached token valid until " + cached.ExpiresAt.UTC().Format(time.RFC3339),
	}
}

func checkAccountConfig(cfg *config.Config) doctorCheck {
	if len(cfg.Accounts) == 0 {
		return doctorCheck{Name: "account_config", Status: "warn", Message: "no account configured"}
	}
	for _, account := range cfg.Accounts {
		if account.AppID == "" || account.AppSecret == "" {
			return doctorCheck{Name: "account_config", Status: "fail", Message: "one or more accounts are missing app_id or app_secret"}
		}
	}
	return doctorCheck{Name: "account_config", Status: "pass", Message: "account credentials are configured"}
}

func checkDefaultAccount(cfg *config.Config) doctorCheck {
	if len(cfg.Accounts) == 0 {
		return doctorCheck{Name: "default_account", Status: "warn", Message: "no default account because no accounts are configured"}
	}
	if cfg.DefaultAccount == "" {
		return doctorCheck{Name: "default_account", Status: "warn", Message: "no default account configured; commands require --account"}
	}
	if _, ok := config.FindAccount(cfg, cfg.DefaultAccount); !ok {
		return doctorCheck{Name: "default_account", Status: "fail", Message: "default account does not exist"}
	}
	return doctorCheck{Name: "default_account", Status: "pass", Message: cfg.DefaultAccount}
}

func checkAPIProxy(cfg *config.Config) doctorCheck {
	if cfg.APIProxy == "" {
		return doctorCheck{Name: "api_proxy", Status: "pass", Message: "direct"}
	}
	if err := validateProxyURL(cfg.APIProxy); err != nil {
		return doctorCheck{Name: "api_proxy", Status: "fail", Message: err.Error()}
	}
	return doctorCheck{Name: "api_proxy", Status: "pass", Message: cfg.APIProxy}
}
