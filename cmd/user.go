package cmd

import (
	"github.com/spf13/cobra"
)

// userInfoBatchCap is the WeChat /cgi-bin/user/info/batchget per-call limit;
// the command auto-chunks longer lists into sequential calls (CLI-SPEC §15.6).
const userInfoBatchCap = 100

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

var userInfoBatch = struct {
	account string
	lang    string
	openids []string
	batch   batchFlags
}{lang: "zh_CN"}

var userInfoBatchCmd = readCommand(&cobra.Command{
	Use:   "info-batch",
	Short: "Get up to 100 follower profiles in one call (auto-chunked)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve plural input: comma-separated or repeated --openids, de-duped,
		// input order preserved so items[] zips back to the request (§15.1).
		openids := parsePluralFlag(userInfoBatch.openids)
		if err := requireTargets(openids, "--openids", 0); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		cfg, account, token, err := accessToken(userInfoBatch.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}

		items := make([]batchItem, 0, len(openids))
		skipped := 0
		// Auto-chunk at the upstream cap; each chunk is one batchget call (§15.6).
		// Chunking is invisible in the contract: one aggregated items[]/summary.
		chunks := chunk(openids, userInfoBatchCap)
		stopped := false
		for _, ch := range chunks {
			if stopped {
				skipped += len(ch)
				continue
			}
			userList := make([]map[string]any, 0, len(ch))
			for _, openid := range ch {
				entry := map[string]any{"openid": openid}
				if userInfoBatch.lang != "" {
					entry["lang"] = userInfoBatch.lang
				}
				userList = append(userList, entry)
			}
			res, callErr := client.GetUserInfoBatch(apiCtx(), token, userList)
			if callErr != nil {
				// A chunk-level failure maps onto every item in that chunk; one
				// failed chunk does not fail the whole command unless
				// --continue-on-error=false (§15.6).
				code, _, retryable, _ := classifyCallError(callErr)
				for _, openid := range ch {
					items = append(items, batchItem{Target: openid, OK: false,
						Error: &batchItemErr{Code: code, Retryable: retryable, Message: callErr.Error()}})
				}
				if !userInfoBatch.batch.continueOnError {
					stopped = true
				}
				continue
			}
			byOpenID := indexUserInfoList(res)
			for _, openid := range ch {
				if profile, ok := byOpenID[openid]; ok {
					items = append(items, batchItem{Target: openid, OK: true, Data: profile})
				} else {
					// Upstream silently drops openids that are not followers; report
					// them per item rather than hiding the gap.
					items = append(items, batchItem{Target: openid, OK: false,
						Error: &batchItemErr{Code: "E_NOT_FOUND", Retryable: false, Message: "openid not returned by batchget (not a follower?)"}})
				}
			}
		}

		summary := summarize(items, skipped)
		return printData(map[string]any{
			"account":    account.Alias,
			"items":      items,
			"summary":    summary,
			"_untrusted": []string{"items"},
		})
	},
}, "user_info_batch")

func init() {
	userInfoCmd.Flags().StringVar(&userInfo.account, "account", "", "Account alias; defaults to configured default")
	userInfoCmd.Flags().StringVar(&userInfo.lang, "lang", "zh_CN", "Profile language: zh_CN|zh_TW|en")
	userListCmd.Flags().StringVar(&userList.account, "account", "", "Account alias; defaults to configured default")
	userListCmd.Flags().StringVar(&userList.nextOpenID, "next-openid", "", "Cursor: start listing after this openid")
	userInfoBatchCmd.Flags().StringVar(&userInfoBatch.account, "account", "", "Account alias; defaults to configured default")
	userInfoBatchCmd.Flags().StringVar(&userInfoBatch.lang, "lang", "zh_CN", "Profile language: zh_CN|zh_TW|en")
	userInfoBatchCmd.Flags().StringArrayVar(&userInfoBatch.openids, "openids", nil, "Follower openids: comma-separated and/or repeated (≤100 per call, auto-chunked)")
	bindBatchFlags(userInfoBatchCmd, &userInfoBatch.batch, true)
	userCmd.AddCommand(userInfoCmd, userListCmd, userInfoBatchCmd)
	rootCmd.AddCommand(userCmd)
}

// indexUserInfoList maps the upstream user_info_list back to openid → profile so
// each input openid can be zipped to its result. WeChat returns numeric and
// string fields under user_info_list.
func indexUserInfoList(res map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	list, _ := res["user_info_list"].([]any)
	for _, entry := range list {
		profile, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		openid, _ := profile["openid"].(string)
		if openid == "" {
			continue
		}
		out[openid] = profile
	}
	return out
}
