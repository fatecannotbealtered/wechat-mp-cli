package cmd

import (
	"errors"
	"net/url"
	"strings"

	"github.com/fatecannotbealtered/wechat-mp-cli/internal/config"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure local WeChat Official Account access",
}

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Manage configured Official Account credentials",
}

var accountAdd = struct {
	alias       string
	appID       string
	secret      string
	secretEnv   string
	makeDefault bool
}{}

var accountAddCmd = writeCommand(&cobra.Command{
	Use:   "add",
	Short: "Add or update an account",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		secret := envOrFlagSecret(accountAdd.secret, accountAdd.secretEnv)
		payload := map[string]any{
			"alias":        accountAdd.alias,
			"app_id":       accountAdd.appID,
			"secret_set":   secret != "",
			"make_default": accountAdd.makeDefault,
		}
		if err := required(accountAdd.alias, "--alias"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		if err := required(accountAdd.appID, "--app-id"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		if err := required(secret, "--secret or --secret-env"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		ok, err := confirmWrite("setup.account.add", payload, payload)
		if err != nil || !ok {
			return err
		}
		cfg, err := loadConfig()
		if err != nil {
			return handleError(err)
		}
		if err := config.AddOrUpdateAccount(cfg, config.Account{
			Alias:     accountAdd.alias,
			AppID:     accountAdd.appID,
			AppSecret: secret,
			Source:    "config",
		}, accountAdd.makeDefault); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		if err := config.Save(cfg); err != nil {
			return handleError(err)
		}
		return printData(map[string]any{
			"saved":           true,
			"config_path":     config.FilePath(),
			"default_account": cfg.DefaultAccount,
			"accounts":        config.PublicAccounts(cfg),
		})
	},
}, "medium", "Writes encrypted app credentials into the local configuration file.")

var accountListCmd = readCommand(&cobra.Command{
	Use:   "list",
	Short: "List configured accounts without exposing secrets",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return handleError(err)
		}
		return printData(map[string]any{
			"default_account": cfg.DefaultAccount,
			"accounts":        config.PublicAccounts(cfg),
		})
	},
}, "account_list")

var accountDefaultAlias string

var accountDefaultCmd = writeCommand(&cobra.Command{
	Use:   "default",
	Short: "Set the default account",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"alias": accountDefaultAlias}
		if err := required(accountDefaultAlias, "--alias"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		ok, err := confirmWrite("setup.account.default", payload, payload)
		if err != nil || !ok {
			return err
		}
		cfg, err := loadConfig()
		if err != nil {
			return handleError(err)
		}
		if err := config.SetDefault(cfg, accountDefaultAlias); err != nil {
			return fail(ExitNotFound, "E_NOT_FOUND", err.Error(), false)
		}
		if err := config.Save(cfg); err != nil {
			return handleError(err)
		}
		return printData(map[string]any{"default_account": cfg.DefaultAccount})
	},
}, "medium", "Changes which configured WeChat account subsequent commands use by default.")

var accountRemoveAlias string

var accountRemoveCmd = writeCommand(&cobra.Command{
	Use:   "remove",
	Short: "Remove an account from local configuration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"alias": accountRemoveAlias}
		if err := required(accountRemoveAlias, "--alias"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		ok, err := confirmWrite("setup.account.remove", payload, payload)
		if err != nil || !ok {
			return err
		}
		cfg, err := loadConfig()
		if err != nil {
			return handleError(err)
		}
		if !config.RemoveAccount(cfg, accountRemoveAlias) {
			return fail(ExitNotFound, "E_NOT_FOUND", "account not found", false)
		}
		if err := config.Save(cfg); err != nil {
			return handleError(err)
		}
		return printData(map[string]any{
			"removed":         true,
			"default_account": cfg.DefaultAccount,
			"accounts":        config.PublicAccounts(cfg),
		})
	},
}, "medium", "Deletes local encrypted credentials for one configured account.")

var accountTestAlias string

var accountTestCmd = readCommand(&cobra.Command{
	Use:   "test",
	Short: "Fetch an access token to verify account credentials",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, account, err := resolveAccount(accountTestAlias)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		token, err := client.StableToken(apiCtx(), account.AppID, account.AppSecret, false)
		if err != nil {
			return handleError(err)
		}
		return printData(map[string]any{
			"account":    account.Alias,
			"app_id":     account.AppID,
			"api_base":   cfg.APIBase,
			"token_ok":   strings.TrimSpace(token.AccessToken) != "",
			"expires_in": token.ExpiresIn,
		})
	},
}, "account_test")

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Manage optional API proxy configuration",
}

var proxySetURL string

var proxySetCmd = writeCommand(&cobra.Command{
	Use:   "set",
	Short: "Set API proxy URL for WeChat API requests",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateProxyURL(proxySetURL); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		payload := map[string]any{"api_proxy": proxySetURL}
		ok, err := confirmWrite("setup.proxy.set", payload, payload)
		if err != nil || !ok {
			return err
		}
		cfg, err := loadConfig()
		if err != nil {
			return handleError(err)
		}
		cfg.APIProxy = strings.TrimSpace(proxySetURL)
		if err := config.Save(cfg); err != nil {
			return handleError(err)
		}
		return printData(map[string]any{
			"saved":       true,
			"api_proxy":   cfg.APIProxy,
			"api_base":    cfg.APIBase,
			"config_path": config.FilePath(),
		})
	},
}, "medium", "Changes the outbound network path used for WeChat API calls.")

var proxyClearCmd = writeCommand(&cobra.Command{
	Use:   "clear",
	Short: "Clear saved API proxy URL",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"api_proxy": ""}
		ok, err := confirmWrite("setup.proxy.clear", payload, payload)
		if err != nil || !ok {
			return err
		}
		cfg, err := loadConfig()
		if err != nil {
			return handleError(err)
		}
		cfg.APIProxy = ""
		if err := config.Save(cfg); err != nil {
			return handleError(err)
		}
		return printData(map[string]any{"saved": true, "api_proxy": ""})
	},
}, "medium", "Removes the saved API proxy and returns API calls to the direct network path.")

var proxyStatusCmd = readCommand(&cobra.Command{
	Use:   "status",
	Short: "Show effective API proxy configuration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return handleError(err)
		}
		return printData(map[string]any{
			"api_base":  cfg.APIBase,
			"api_proxy": cfg.APIProxy,
			"enabled":   strings.TrimSpace(cfg.APIProxy) != "",
			"examples": []string{
				"ssh -N -D 127.0.0.1:1080 user@server.example.com",
				"WECHAT_MP_CLI_API_PROXY=socks5://127.0.0.1:1080 wechat-mp-cli setup account test",
			},
		})
	},
}, "proxy_status")

func init() {
	accountAddCmd.Flags().StringVar(&accountAdd.alias, "alias", "", "Local account alias")
	accountAddCmd.Flags().StringVar(&accountAdd.appID, "app-id", "", "WeChat Official Account AppID")
	accountAddCmd.Flags().StringVar(&accountAdd.secret, "secret", "", "WeChat Official Account AppSecret")
	accountAddCmd.Flags().StringVar(&accountAdd.secretEnv, "secret-env", "", "Environment variable that contains the AppSecret")
	accountAddCmd.Flags().BoolVar(&accountAdd.makeDefault, "default", false, "Make this account the default")

	accountDefaultCmd.Flags().StringVar(&accountDefaultAlias, "alias", "", "Account alias to make default")
	accountRemoveCmd.Flags().StringVar(&accountRemoveAlias, "alias", "", "Account alias to remove")
	accountTestCmd.Flags().StringVar(&accountTestAlias, "account", "", "Account alias; defaults to configured default")

	proxySetCmd.Flags().StringVar(&proxySetURL, "url", "", "Proxy URL: http://, https://, socks5://, or socks5h://")
	proxyCmd.AddCommand(proxySetCmd, proxyClearCmd, proxyStatusCmd)

	accountCmd.AddCommand(accountAddCmd, accountListCmd, accountDefaultCmd, accountRemoveCmd, accountTestCmd)
	setupCmd.AddCommand(accountCmd, proxyCmd)
	rootCmd.AddCommand(setupCmd)
}

func validateProxyURL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return required(value, "--url")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return err
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks5", "socks5h":
	default:
		return errors.New("proxy URL scheme must be http, https, socks5, or socks5h")
	}
	if parsed.Host == "" {
		return errors.New("proxy URL host is required")
	}
	return nil
}
