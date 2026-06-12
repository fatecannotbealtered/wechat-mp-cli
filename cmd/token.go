package cmd

import (
	"time"

	"github.com/fatecannotbealtered/wechat-mp-cli/internal/config"
	"github.com/fatecannotbealtered/wechat-mp-cli/internal/output"
	"github.com/spf13/cobra"
)

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Inspect or refresh WeChat API access tokens",
}

var tokenStatusAccount string

var tokenStatusCmd = readCommand(&cobra.Command{
	Use:   "status",
	Short: "Show whether credentials are available for token refresh",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return handleError(err)
		}
		account, err := config.ResolveAccount(cfg, tokenStatusAccount)
		if err != nil {
			return printData(map[string]any{
				"configured": false,
				"error":      err.Error(),
				"config":     config.FilePath(),
			})
		}
		result := map[string]any{
			"configured":  true,
			"account":     account.Alias,
			"app_id":      account.AppID,
			"api_base":    cfg.APIBase,
			"refreshable": account.AppSecret != "",
		}
		if cached, ok := config.LoadCachedToken(account.AppID); ok {
			result["cached"] = true
			result["valid"] = time.Until(cached.ExpiresAt) > 0
			result["expires_at"] = cached.ExpiresAt.UTC().Format(time.RFC3339)
		} else {
			result["cached"] = false
			result["valid"] = false
		}
		return printData(result)
	},
}, "token_status")

var tokenRefreshAccount string

var tokenRefreshCmd = readCommand(&cobra.Command{
	Use:   "refresh",
	Short: "Fetch a fresh access token without printing the secret token",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, account, err := resolveAccount(tokenRefreshAccount)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		token, err := client.StableToken(apiCtx(), account.AppID, account.AppSecret, true)
		if err != nil {
			return handleError(err)
		}
		expiresAt := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second).UTC()
		if err := config.SaveCachedToken(account.AppID, token.AccessToken, expiresAt); err != nil {
			output.Error("warning: token cache not persisted: %v", err)
		}
		return printData(map[string]any{
			"account":     account.Alias,
			"app_id":      account.AppID,
			"api_base":    cfg.APIBase,
			"token_ok":    token.AccessToken != "",
			"expires_in":  token.ExpiresIn,
			"expires_at":  expiresAt.Format(time.RFC3339),
			"token_value": "redacted",
		})
	},
}, "token_refresh")

func init() {
	tokenStatusCmd.Flags().StringVar(&tokenStatusAccount, "account", "", "Account alias; defaults to configured default")
	tokenRefreshCmd.Flags().StringVar(&tokenRefreshAccount, "account", "", "Account alias; defaults to configured default")
	tokenCmd.AddCommand(tokenStatusCmd, tokenRefreshCmd)
	rootCmd.AddCommand(tokenCmd)
}
