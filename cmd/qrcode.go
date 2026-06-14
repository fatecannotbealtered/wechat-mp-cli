package cmd

import (
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

// showQRCodePrefix renders a scannable QR image from a ticket; the ticket must
// be URL-encoded because it can contain characters outside the unreserved set.
const showQRCodePrefix = "https://mp.weixin.qq.com/cgi-bin/showqrcode?ticket="

var qrcodeCmd = &cobra.Command{
	Use:   "qrcode",
	Short: "Manage WeChat account QR codes",
}

var qrcodeCreate = struct {
	account       string
	expireSeconds int
	sceneID       int
	sceneStr      string
}{}

var qrcodeCreateCmd = writeCommand(&cobra.Command{
	Use:   "create",
	Short: "Create a QR code ticket (temporary with --expire-seconds, otherwise permanent)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		sceneStr := strings.TrimSpace(qrcodeCreate.sceneStr)
		// Exactly one scene source: numeric scene_id or string scene_str.
		if (sceneStr == "") == (qrcodeCreate.sceneID == 0) {
			return fail(ExitBadArgs, "E_VALIDATION", "exactly one of --scene-id (non-zero) or --scene-str is required", false)
		}
		temporary := qrcodeCreate.expireSeconds > 0

		// action_name picks temporary vs permanent and numeric vs string scene.
		var actionName string
		scene := map[string]any{}
		switch {
		case temporary && sceneStr != "":
			actionName, scene["scene_str"] = "QR_STR_SCENE", sceneStr
		case temporary:
			actionName, scene["scene_id"] = "QR_SCENE", qrcodeCreate.sceneID
		case sceneStr != "":
			actionName, scene["scene_str"] = "QR_LIMIT_STR_SCENE", sceneStr
		default:
			actionName, scene["scene_id"] = "QR_LIMIT_SCENE", qrcodeCreate.sceneID
		}
		apiPayload := map[string]any{
			"action_name": actionName,
			"action_info": map[string]any{"scene": scene},
		}
		if temporary {
			apiPayload["expire_seconds"] = qrcodeCreate.expireSeconds
		}

		confirmPayload := map[string]any{
			"account":        qrcodeCreate.account,
			"action_name":    actionName,
			"scene_id":       qrcodeCreate.sceneID,
			"scene_str":      sceneStr,
			"expire_seconds": qrcodeCreate.expireSeconds,
		}
		ok, err := confirmWrite("qrcode.create", confirmPayload, map[string]any{
			"account":        qrcodeCreate.account,
			"action_name":    actionName,
			"permanent":      !temporary,
			"expire_seconds": qrcodeCreate.expireSeconds,
			"will_call_api":  true,
			"api_operation":  "qrcode/create",
		})
		if err != nil || !ok {
			return err
		}
		cfg, account, token, err := accessToken(qrcodeCreate.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.CreateQRCode(apiCtx(), token, apiPayload)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		res["action_name"] = actionName
		if ticket, _ := res["ticket"].(string); ticket != "" {
			res["showqrcode_url"] = showQRCodePrefix + url.QueryEscape(ticket)
		}
		return printData(res)
	},
}, "medium", "Creates a persistent QR code ticket for the configured WeChat account.")

func init() {
	qrcodeCreateCmd.Flags().StringVar(&qrcodeCreate.account, "account", "", "Account alias; defaults to configured default")
	qrcodeCreateCmd.Flags().IntVar(&qrcodeCreate.expireSeconds, "expire-seconds", 0, "Temporary QR lifetime in seconds (max 2592000); 0 creates a permanent QR")
	qrcodeCreateCmd.Flags().IntVar(&qrcodeCreate.sceneID, "scene-id", 0, "Numeric scene id (QR_SCENE/QR_LIMIT_SCENE)")
	qrcodeCreateCmd.Flags().StringVar(&qrcodeCreate.sceneStr, "scene-str", "", "String scene value (QR_STR_SCENE/QR_LIMIT_STR_SCENE)")
	qrcodeCmd.AddCommand(qrcodeCreateCmd)
	rootCmd.AddCommand(qrcodeCmd)
}
