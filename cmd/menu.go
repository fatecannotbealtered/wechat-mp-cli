package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var menuCmd = &cobra.Command{
	Use:   "menu",
	Short: "Manage WeChat custom menus",
}

var menuAccount string

var menuGetCmd = readCommand(&cobra.Command{
	Use:   "get",
	Short: "Get the current self menu info",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, account, token, err := accessToken(menuAccount)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.GetCurrentSelfMenu(apiCtx(), token)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		return printData(res)
	},
}, "menu")

var menuSet = struct {
	account string
	file    string
}{}

var menuSetCmd = writeCommand(&cobra.Command{
	Use:   "set",
	Short: "Set custom menu from a JSON file",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := required(menuSet.file, "--file"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		data, err := os.ReadFile(menuSet.file)
		if err != nil {
			return handleError(err)
		}
		if !json.Valid(data) {
			return fail(ExitBadArgs, "E_VALIDATION", "--file must contain valid JSON", false)
		}
		payload := map[string]any{
			"account":   menuSet.account,
			"file":      filepath.Clean(menuSet.file),
			"json_hash": sha256Bytes(data),
		}
		ok, err := confirmWrite("menu.set", payload, map[string]any{
			"file":          filepath.Clean(menuSet.file),
			"json_hash":     sha256Bytes(data),
			"will_call_api": true,
			"api_operation": "menu/create",
		})
		if err != nil || !ok {
			return err
		}
		cfg, account, token, err := accessToken(menuSet.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.CreateMenu(apiCtx(), token, data)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		return printData(res)
	},
}, "high", "Replaces the configured WeChat account custom menu.")

var menuDeleteAccount string

var menuDeleteCmd = writeCommand(&cobra.Command{
	Use:   "delete",
	Short: "Delete the custom menu",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"account": menuDeleteAccount}
		ok, err := confirmWrite("menu.delete", payload, payload)
		if err != nil || !ok {
			return err
		}
		cfg, account, token, err := accessToken(menuDeleteAccount)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.DeleteMenu(apiCtx(), token)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		return printData(res)
	},
}, "high", "Deletes the configured WeChat account custom menu.")

func init() {
	menuGetCmd.Flags().StringVar(&menuAccount, "account", "", "Account alias; defaults to configured default")
	menuSetCmd.Flags().StringVar(&menuSet.account, "account", "", "Account alias; defaults to configured default")
	menuSetCmd.Flags().StringVar(&menuSet.file, "file", "", "Menu JSON file")
	menuDeleteCmd.Flags().StringVar(&menuDeleteAccount, "account", "", "Account alias; defaults to configured default")
	menuCmd.AddCommand(menuGetCmd, menuSetCmd, menuDeleteCmd)
	rootCmd.AddCommand(menuCmd)
}

func sha256Bytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
