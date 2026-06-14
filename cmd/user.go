package cmd

import (
	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Inspect Official Account followers",
}

var userInfo = struct {
	account string
	lang    string
}{lang: "zh_CN"}

var userInfoCmd = readCommand(&cobra.Command{
	Use:   "info <openid>",
	Short: "Get a single follower profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := required(args[0], "<openid>"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		cfg, account, token, err := accessToken(userInfo.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.GetUserInfo(apiCtx(), token, args[0], userInfo.lang)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		// nickname/remark are user-controlled; tag whichever are present.
		return printData(markUntrusted(res, "nickname", "remark"))
	},
}, "user_info")

var userList = struct {
	account    string
	nextOpenID string
}{}

var userListCmd = readCommand(&cobra.Command{
	Use:   "list",
	Short: "List follower openids (cursor: next_openid)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, account, token, err := accessToken(userList.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.ListUsers(apiCtx(), token, userList.nextOpenID)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		return printData(res)
	},
}, "user_list")

func init() {
	userInfoCmd.Flags().StringVar(&userInfo.account, "account", "", "Account alias; defaults to configured default")
	userInfoCmd.Flags().StringVar(&userInfo.lang, "lang", "zh_CN", "Profile language: zh_CN|zh_TW|en")
	userListCmd.Flags().StringVar(&userList.account, "account", "", "Account alias; defaults to configured default")
	userListCmd.Flags().StringVar(&userList.nextOpenID, "next-openid", "", "Cursor: start listing after this openid")
	userCmd.AddCommand(userInfoCmd, userListCmd)
	rootCmd.AddCommand(userCmd)
}
