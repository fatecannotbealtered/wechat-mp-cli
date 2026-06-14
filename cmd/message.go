package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// massSendOpenIDCap is the WeChat /cgi-bin/message/mass/send per-call limit. The
// list is delivered as one mass job (not chunked) — beyond the cap is a usage
// error, not a silent split, because a mass broadcast is a single async job.
const massSendOpenIDCap = 10000

var messageCmd = &cobra.Command{
	Use:   "message",
	Short: "Send and inspect mass (broadcast) messages",
}

var messageMassCmd = &cobra.Command{
	Use:   "mass",
	Short: "Mass-send broadcasts to tags, all followers, or an openid list",
}

// massBody holds the shared message-body flags. WeChat mass send supports
// several media types; this CLI exposes the two common ones: mpnews (a published
// article, by media_id) and text (inline content). Exactly one must be set.
type massBody struct {
	mpnewsMediaID string
	text          string
}

// resolveMassMessage builds the {msgtype: {...}} fragment WeChat expects. It is
// a boundary guard: exactly one body type is required.
func resolveMassMessage(b massBody) (map[string]any, string, error) {
	mpnews := strings.TrimSpace(b.mpnewsMediaID)
	text := strings.TrimSpace(b.text)
	switch {
	case mpnews != "" && text != "":
		return nil, "", fmt.Errorf("set only one of --mpnews-media-id or --text")
	case mpnews != "":
		return map[string]any{"mpnews": map[string]any{"media_id": mpnews}, "msgtype": "mpnews"}, "mpnews", nil
	case text != "":
		return map[string]any{"text": map[string]any{"content": text}, "msgtype": "text"}, "text", nil
	default:
		return nil, "", fmt.Errorf("a message body is required: set --mpnews-media-id or --text")
	}
}

var massSendAll = struct {
	account string
	tagID   int
	toAll   bool
	body    massBody
}{}

var massSendAllCmd = writeCommand(&cobra.Command{
	Use:   "sendall",
	Short: "Broadcast to all followers or a single tag (asynchronous)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		body, msgType, err := resolveMassMessage(massSendAll.body)
		if err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		// Audience filter: --to-all reaches every follower; otherwise a tag is
		// required so a broadcast can never default to the whole base by accident.
		filter := map[string]any{"is_to_all": massSendAll.toAll}
		if !massSendAll.toAll {
			if massSendAll.tagID <= 0 {
				return fail(ExitBadArgs, "E_VALIDATION", "set --to-all or a positive --tag-id", false)
			}
			filter["tag_id"] = massSendAll.tagID
		}
		payload := map[string]any{"account": massSendAll.account, "filter": filter}
		for k, v := range body {
			payload[k] = v
		}
		audience := fmt.Sprintf("tag_id=%d", massSendAll.tagID)
		if massSendAll.toAll {
			audience = "all followers"
		}
		ok, err := confirmWrite("message.mass.sendall", payload, map[string]any{
			"audience":      audience,
			"msgtype":       msgType,
			"will_call_api": true,
			"api_operation": "message/mass/sendall",
			"note":          "Mass send is asynchronous and reaches real followers; poll with message mass get --msg-id.",
		})
		if err != nil || !ok {
			return err
		}
		cfg, account, token, err := accessToken(massSendAll.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.MassSendAll(apiCtx(), token, payload)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		res["audience"] = audience
		res["status_hint"] = "run message mass get --msg-id <msg_id>"
		return printData(res)
	},
}, "critical", "Broadcasts a message to all followers or a tag of the Official Account.")

var massSend = struct {
	account string
	openids []string
	body    massBody
}{}

var massSendCmd = writeCommand(&cobra.Command{
	Use:   "send",
	Short: "Broadcast to an explicit openid list (≤10000, one async job)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Plural input: comma-separated and/or repeated, de-duped, order preserved.
		openids := parsePluralFlag(massSend.openids)
		if err := requireTargets(openids, "--openids", massSendOpenIDCap); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		body, msgType, err := resolveMassMessage(massSend.body)
		if err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		payload := map[string]any{"account": massSend.account, "touser": openids}
		for k, v := range body {
			payload[k] = v
		}
		ok, err := confirmWrite("message.mass.send", payload, map[string]any{
			"action":        "mass_send",
			"total":         len(openids),
			"targets":       openids,
			"msgtype":       msgType,
			"will_call_api": true,
			"api_operation": "message/mass/send",
			"note":          "One asynchronous mass job for the whole openid list; not atomic per recipient. Poll with message mass get --msg-id.",
		})
		if err != nil || !ok {
			return err
		}
		cfg, account, token, err := accessToken(massSend.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.MassSend(apiCtx(), token, payload)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		res["total"] = len(openids)
		res["status_hint"] = "run message mass get --msg-id <msg_id>"
		return printData(res)
	},
}, "critical", "Broadcasts a message to an explicit list of up to 10000 followers.")

var massPreview = struct {
	account  string
	openid   string
	towxname string
	body     massBody
}{}

var massPreviewCmd = writeCommand(&cobra.Command{
	Use:   "preview",
	Short: "Send a single preview of a mass message to one recipient",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		openid := strings.TrimSpace(massPreview.openid)
		towxname := strings.TrimSpace(massPreview.towxname)
		if openid == "" && towxname == "" {
			return fail(ExitBadArgs, "E_VALIDATION", "set --openid or --towxname for the preview recipient", false)
		}
		body, msgType, err := resolveMassMessage(massPreview.body)
		if err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		payload := map[string]any{"account": massPreview.account}
		if openid != "" {
			payload["touser"] = openid
		}
		if towxname != "" {
			payload["towxname"] = towxname
		}
		for k, v := range body {
			payload[k] = v
		}
		ok, err := confirmWrite("message.mass.preview", payload, map[string]any{
			"recipient":     strings.TrimSpace(openid + towxname),
			"msgtype":       msgType,
			"will_call_api": true,
			"api_operation": "message/mass/preview",
			"note":          "Sends one real message to the preview recipient only.",
		})
		if err != nil || !ok {
			return err
		}
		cfg, account, token, err := accessToken(massPreview.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.MassPreview(apiCtx(), token, payload)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		return printData(res)
	},
}, "high", "Sends one preview copy of a mass message to a single recipient.")

var massGet = struct {
	account string
	msgID   int64
}{}

var massGetCmd = readCommand(&cobra.Command{
	Use:   "get",
	Short: "Read the delivery status of a mass-send job",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requirePositiveInt64(massGet.msgID, "--msg-id"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		cfg, account, token, err := accessToken(massGet.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.MassGet(apiCtx(), token, massGet.msgID)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		return printData(res)
	},
}, "mass_status")

var massDelete = struct {
	account    string
	msgID      int64
	articleIdx int
}{}

var massDeleteCmd = writeCommand(&cobra.Command{
	Use:   "delete",
	Short: "Delete (recall) a sent mass message",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requirePositiveInt64(massDelete.msgID, "--msg-id"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		payload := map[string]any{"account": massDelete.account, "msg_id": massDelete.msgID, "article_idx": massDelete.articleIdx}
		ok, err := confirmWrite("message.mass.delete", payload, map[string]any{
			"msg_id":        massDelete.msgID,
			"article_idx":   massDelete.articleIdx,
			"will_call_api": true,
			"api_operation": "message/mass/delete",
			"note":          "Recalls a sent mass message; only the article content is removed, delivery already happened.",
		})
		if err != nil || !ok {
			return err
		}
		cfg, account, token, err := accessToken(massDelete.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.MassDelete(apiCtx(), token, massDelete.msgID, massDelete.articleIdx)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		res["msg_id"] = massDelete.msgID
		return printData(res)
	},
}, "critical", "Recalls a sent mass message from the Official Account.")

func init() {
	bindMassBody(massSendAllCmd, &massSendAll.body)
	massSendAllCmd.Flags().StringVar(&massSendAll.account, "account", "", "Account alias; defaults to configured default")
	massSendAllCmd.Flags().IntVar(&massSendAll.tagID, "tag-id", 0, "Send to followers carrying this tag id")
	massSendAllCmd.Flags().BoolVar(&massSendAll.toAll, "to-all", false, "Send to all followers instead of a tag")

	bindMassBody(massSendCmd, &massSend.body)
	massSendCmd.Flags().StringVar(&massSend.account, "account", "", "Account alias; defaults to configured default")
	massSendCmd.Flags().StringArrayVar(&massSend.openids, "openids", nil, "Recipient openids: comma-separated and/or repeated (≤10000, one job)")

	bindMassBody(massPreviewCmd, &massPreview.body)
	massPreviewCmd.Flags().StringVar(&massPreview.account, "account", "", "Account alias; defaults to configured default")
	massPreviewCmd.Flags().StringVar(&massPreview.openid, "openid", "", "Preview recipient openid")
	massPreviewCmd.Flags().StringVar(&massPreview.towxname, "towxname", "", "Preview recipient WeChat id (alternative to --openid)")

	massGetCmd.Flags().StringVar(&massGet.account, "account", "", "Account alias; defaults to configured default")
	massGetCmd.Flags().Int64Var(&massGet.msgID, "msg-id", 0, "Mass message msg_id returned by sendall/send")

	massDeleteCmd.Flags().StringVar(&massDelete.account, "account", "", "Account alias; defaults to configured default")
	massDeleteCmd.Flags().Int64Var(&massDelete.msgID, "msg-id", 0, "Mass message msg_id to recall")
	massDeleteCmd.Flags().IntVar(&massDelete.articleIdx, "article-idx", 0, "Article index inside an mpnews set (0 = first)")

	messageMassCmd.AddCommand(massSendAllCmd, massSendCmd, massPreviewCmd, massGetCmd, massDeleteCmd)
	messageCmd.AddCommand(messageMassCmd)
	rootCmd.AddCommand(messageCmd)
}

func bindMassBody(cmd *cobra.Command, b *massBody) {
	cmd.Flags().StringVar(&b.mpnewsMediaID, "mpnews-media-id", "", "Published-article media_id to broadcast (mpnews)")
	cmd.Flags().StringVar(&b.text, "text", "", "Plain-text content to broadcast")
}
