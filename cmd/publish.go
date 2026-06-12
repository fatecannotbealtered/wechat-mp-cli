package cmd

import "github.com/spf13/cobra"

var publishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Submit drafts for publication and inspect publish jobs",
}

var publishSubmit = struct {
	account string
	mediaID string
}{}

var publishSubmitCmd = writeCommand(&cobra.Command{
	Use:   "submit",
	Short: "Submit a draft media_id for publication",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := required(publishSubmit.mediaID, "--media-id"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		payload := map[string]any{"account": publishSubmit.account, "media_id": publishSubmit.mediaID}
		ok, err := confirmWrite("publish.submit", payload, map[string]any{
			"media_id":      publishSubmit.mediaID,
			"will_call_api": true,
			"api_operation": "freepublish/submit",
			"note":          "WeChat publication is asynchronous; use publish status with publish_id.",
		})
		if err != nil || !ok {
			return err
		}
		cfg, account, token, err := accessToken(publishSubmit.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.SubmitPublish(apiCtx(), token, publishSubmit.mediaID)
		if err != nil {
			return handleError(err)
		}
		return printData(map[string]any{
			"account":     account.Alias,
			"media_id":    publishSubmit.mediaID,
			"publish_id":  res.PublishID,
			"msg_data_id": res.MsgDataID,
			"status_hint": "run publish status --publish-id <publish_id>",
		})
	},
}, "critical", "Submits a WeChat draft for public publication.")

var publishStatus = struct {
	account   string
	publishID string
}{}

var publishStatusCmd = readCommand(&cobra.Command{
	Use:   "status",
	Short: "Read asynchronous publication status",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := required(publishStatus.publishID, "--publish-id"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		cfg, account, token, err := accessToken(publishStatus.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.GetPublishStatus(apiCtx(), token, publishStatus.publishID)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		return printData(res)
	},
}, "publish_status")

var publishList = struct {
	account   string
	offset    int
	count     int
	noContent bool
}{count: 20, noContent: true}

var publishListCmd = readCommand(&cobra.Command{
	Use:   "list",
	Short: "List successfully published articles",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, account, token, err := accessToken(publishList.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.BatchGetPublish(apiCtx(), token, publishList.offset, publishList.count, publishList.noContent)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		return printData(markUntrusted(res, "item"))
	},
}, "publish_list")

var publishArticle = struct {
	account   string
	articleID string
}{}

var publishArticleCmd = readCommand(&cobra.Command{
	Use:   "get-article",
	Short: "Get a successfully published article by article_id",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := required(publishArticle.articleID, "--article-id"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		cfg, account, token, err := accessToken(publishArticle.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.GetPublishedArticle(apiCtx(), token, publishArticle.articleID)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		return printData(markUntrusted(res, "news_item"))
	},
}, "published_article")

var publishDelete = struct {
	account   string
	articleID string
	index     int
}{}

var publishDeleteCmd = writeCommand(&cobra.Command{
	Use:   "delete",
	Short: "Delete a successfully published article",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := required(publishDelete.articleID, "--article-id"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		payload := map[string]any{"account": publishDelete.account, "article_id": publishDelete.articleID, "index": publishDelete.index}
		ok, err := confirmWrite("publish.delete", payload, map[string]any{
			"article_id":    publishDelete.articleID,
			"index":         publishDelete.index,
			"will_call_api": true,
			"api_operation": "freepublish/delete",
		})
		if err != nil || !ok {
			return err
		}
		cfg, account, token, err := accessToken(publishDelete.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.DeletePublishedArticle(apiCtx(), token, publishDelete.articleID, publishDelete.index)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		res["article_id"] = publishDelete.articleID
		res["index"] = publishDelete.index
		return printData(res)
	},
}, "critical", "Deletes a successfully published WeChat article.")

func init() {
	publishSubmitCmd.Flags().StringVar(&publishSubmit.account, "account", "", "Account alias; defaults to configured default")
	publishSubmitCmd.Flags().StringVar(&publishSubmit.mediaID, "media-id", "", "Draft media_id to publish")
	publishStatusCmd.Flags().StringVar(&publishStatus.account, "account", "", "Account alias; defaults to configured default")
	publishStatusCmd.Flags().StringVar(&publishStatus.publishID, "publish-id", "", "Publish job id returned by publish submit")
	publishListCmd.Flags().StringVar(&publishList.account, "account", "", "Account alias; defaults to configured default")
	publishListCmd.Flags().IntVar(&publishList.offset, "offset", 0, "Published article list offset")
	publishListCmd.Flags().IntVar(&publishList.count, "count", 20, "Published article list count")
	publishListCmd.Flags().BoolVar(&publishList.noContent, "no-content", true, "Omit article content in published article list")
	publishArticleCmd.Flags().StringVar(&publishArticle.account, "account", "", "Account alias; defaults to configured default")
	publishArticleCmd.Flags().StringVar(&publishArticle.articleID, "article-id", "", "Published article_id")
	publishDeleteCmd.Flags().StringVar(&publishDelete.account, "account", "", "Account alias; defaults to configured default")
	publishDeleteCmd.Flags().StringVar(&publishDelete.articleID, "article-id", "", "Published article_id")
	publishDeleteCmd.Flags().IntVar(&publishDelete.index, "index", 0, "Article index inside the published article set")
	publishCmd.AddCommand(publishSubmitCmd, publishStatusCmd, publishListCmd, publishArticleCmd, publishDeleteCmd)
	rootCmd.AddCommand(publishCmd)
}
