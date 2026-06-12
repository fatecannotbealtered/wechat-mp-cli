package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fatecannotbealtered/wechat-mp-cli/internal/mpapi"
	"github.com/fatecannotbealtered/wechat-mp-cli/internal/output"
	"github.com/spf13/cobra"
)

var assetCmd = &cobra.Command{
	Use:   "asset",
	Short: "Manage WeChat permanent materials",
}

var assetAccount string

var assetCountCmd = readCommand(&cobra.Command{
	Use:   "count",
	Short: "Get permanent material counts",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, account, token, err := accessToken(assetAccount)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.MaterialCount(apiCtx(), token)
		if err != nil {
			return handleError(err)
		}
		return printData(map[string]any{
			"account": account.Alias,
			"counts":  res,
		})
	},
}, "asset_count")

var assetList = struct {
	account      string
	materialType string
	offset       int
	count        int
}{materialType: "image", count: 20}

var assetListCmd = readCommand(&cobra.Command{
	Use:   "list",
	Short: "List permanent materials by type",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		materialType := strings.ToLower(strings.TrimSpace(assetList.materialType))
		if !validMaterialType(materialType) {
			return fail(ExitBadArgs, "E_VALIDATION", "--type must be image, voice, video, or news", false)
		}
		cfg, account, token, err := accessToken(assetList.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.BatchGetMaterial(apiCtx(), token, materialType, assetList.offset, assetList.count)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		res["type"] = materialType
		return printData(markUntrusted(res, "item"))
	},
}, "asset_list")

var assetGet = struct {
	account   string
	mediaID   string
	output    string
	overwrite bool
}{}

var assetGetCmd = readCommand(&cobra.Command{
	Use:   "get",
	Short: "Get a permanent material by media_id",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := required(assetGet.mediaID, "--media-id"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		cfg, account, token, err := accessToken(assetGet.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.GetMaterial(apiCtx(), token, assetGet.mediaID)
		if err != nil {
			return handleError(err)
		}
		return printOrSaveMedia(account.Alias, assetGet.mediaID, assetGet.output, assetGet.overwrite, res)
	},
}, "asset")

var assetDelete = struct {
	account string
	mediaID string
}{}

var assetDeleteCmd = writeCommand(&cobra.Command{
	Use:   "delete",
	Short: "Delete a permanent material by media_id",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := required(assetDelete.mediaID, "--media-id"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		payload := map[string]any{"account": assetDelete.account, "media_id": assetDelete.mediaID}
		ok, err := confirmWrite("asset.delete", payload, payload)
		if err != nil || !ok {
			return err
		}
		cfg, account, token, err := accessToken(assetDelete.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.DeleteMaterial(apiCtx(), token, assetDelete.mediaID)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		res["deleted_media_id"] = assetDelete.mediaID
		return printData(res)
	},
}, "high", "Deletes a permanent material from the configured WeChat account.")

var assetTempCmd = &cobra.Command{
	Use:   "temp",
	Short: "Manage temporary media",
}

var assetTempUpload = struct {
	account   string
	mediaType string
}{mediaType: "image"}

var assetTempUploadCmd = writeCommand(&cobra.Command{
	Use:   "upload <file>",
	Short: "Upload temporary media",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mediaType := strings.ToLower(strings.TrimSpace(assetTempUpload.mediaType))
		if !validTemporaryMediaType(mediaType) {
			return fail(ExitBadArgs, "E_VALIDATION", "--type must be image, voice, video, or thumb", false)
		}
		payload := map[string]any{
			"account": assetTempUpload.account,
			"path":    filepath.Clean(args[0]),
			"type":    mediaType,
		}
		ok, err := confirmWrite("asset.temp.upload", payload, payload)
		if err != nil || !ok {
			return err
		}
		cfg, account, token, err := accessToken(assetTempUpload.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.UploadTemporaryMedia(apiCtx(), token, args[0], mediaType)
		if err != nil {
			return handleError(err)
		}
		return printData(map[string]any{
			"account":    account.Alias,
			"type":       res.Type,
			"media_id":   res.MediaID,
			"created_at": res.CreatedAt,
		})
	},
}, "medium", "Uploads temporary media to the configured WeChat account.")

var assetTempGet = struct {
	account   string
	mediaID   string
	output    string
	overwrite bool
}{}

var assetTempGetCmd = readCommand(&cobra.Command{
	Use:   "get",
	Short: "Download temporary media by media_id",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := required(assetTempGet.mediaID, "--media-id"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		cfg, account, token, err := accessToken(assetTempGet.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.GetTemporaryMedia(apiCtx(), token, assetTempGet.mediaID, false)
		if err != nil {
			return handleError(err)
		}
		return printOrSaveMedia(account.Alias, assetTempGet.mediaID, assetTempGet.output, assetTempGet.overwrite, res)
	},
}, "asset")

var assetTempHDVoice = struct {
	account   string
	mediaID   string
	output    string
	overwrite bool
}{}

var assetTempHDVoiceCmd = readCommand(&cobra.Command{
	Use:   "get-hd-voice",
	Short: "Download HD voice media uploaded through JSSDK",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := required(assetTempHDVoice.mediaID, "--media-id"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		cfg, account, token, err := accessToken(assetTempHDVoice.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.GetTemporaryMedia(apiCtx(), token, assetTempHDVoice.mediaID, true)
		if err != nil {
			return handleError(err)
		}
		return printOrSaveMedia(account.Alias, assetTempHDVoice.mediaID, assetTempHDVoice.output, assetTempHDVoice.overwrite, res)
	},
}, "asset")

func init() {
	assetCountCmd.Flags().StringVar(&assetAccount, "account", "", "Account alias; defaults to configured default")
	assetListCmd.Flags().StringVar(&assetList.account, "account", "", "Account alias; defaults to configured default")
	assetListCmd.Flags().StringVar(&assetList.materialType, "type", "image", "Material type: image|voice|video|news")
	assetListCmd.Flags().IntVar(&assetList.offset, "offset", 0, "Material list offset")
	assetListCmd.Flags().IntVar(&assetList.count, "count", 20, "Material list count")
	assetGetCmd.Flags().StringVar(&assetGet.account, "account", "", "Account alias; defaults to configured default")
	assetGetCmd.Flags().StringVar(&assetGet.mediaID, "media-id", "", "Permanent material media_id")
	assetGetCmd.Flags().StringVar(&assetGet.output, "output", "", "Output file for binary material responses")
	assetGetCmd.Flags().BoolVar(&assetGet.overwrite, "overwrite", false, "Overwrite --output if it already exists")
	assetDeleteCmd.Flags().StringVar(&assetDelete.account, "account", "", "Account alias; defaults to configured default")
	assetDeleteCmd.Flags().StringVar(&assetDelete.mediaID, "media-id", "", "Permanent material media_id")

	assetTempUploadCmd.Flags().StringVar(&assetTempUpload.account, "account", "", "Account alias; defaults to configured default")
	assetTempUploadCmd.Flags().StringVar(&assetTempUpload.mediaType, "type", "image", "Temporary media type: image|voice|video|thumb")
	assetTempGetCmd.Flags().StringVar(&assetTempGet.account, "account", "", "Account alias; defaults to configured default")
	assetTempGetCmd.Flags().StringVar(&assetTempGet.mediaID, "media-id", "", "Temporary media media_id")
	assetTempGetCmd.Flags().StringVar(&assetTempGet.output, "output", "", "Output file for binary media responses")
	assetTempGetCmd.Flags().BoolVar(&assetTempGet.overwrite, "overwrite", false, "Overwrite --output if it already exists")
	assetTempHDVoiceCmd.Flags().StringVar(&assetTempHDVoice.account, "account", "", "Account alias; defaults to configured default")
	assetTempHDVoiceCmd.Flags().StringVar(&assetTempHDVoice.mediaID, "media-id", "", "JSSDK uploadVoice serverID/media_id")
	assetTempHDVoiceCmd.Flags().StringVar(&assetTempHDVoice.output, "output", "", "Output file for binary media responses")
	assetTempHDVoiceCmd.Flags().BoolVar(&assetTempHDVoice.overwrite, "overwrite", false, "Overwrite --output if it already exists")

	assetTempCmd.AddCommand(assetTempUploadCmd, assetTempGetCmd, assetTempHDVoiceCmd)
	assetCmd.AddCommand(assetCountCmd, assetListCmd, assetGetCmd, assetDeleteCmd, assetTempCmd)
	rootCmd.AddCommand(assetCmd)
}

func validMaterialType(value string) bool {
	switch value {
	case "image", "voice", "video", "news":
		return true
	default:
		return false
	}
}

func validTemporaryMediaType(value string) bool {
	switch value {
	case "image", "voice", "video", "thumb":
		return true
	default:
		return false
	}
}

func printOrSaveMedia(accountAlias, mediaID, outputPath string, overwrite bool, res *mpapi.MediaDownloadResponse) error {
	if res.JSON != nil {
		res.JSON["account"] = accountAlias
		res.JSON["media_id"] = mediaID
		return printData(res.JSON)
	}
	if outputPath == "" {
		return fail(ExitBadArgs, output.ErrValidation, "response is binary; pass --output to save it", false)
	}
	cleanPath := filepath.Clean(outputPath)
	if !overwrite {
		if _, err := os.Stat(cleanPath); err == nil {
			return fail(ExitConflict, output.ErrConflict, cleanPath+" already exists; pass --overwrite to replace it", false)
		} else if !os.IsNotExist(err) {
			return handleError(err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return handleError(err)
	}
	if err := os.WriteFile(cleanPath, res.Content, 0o644); err != nil {
		return handleError(err)
	}
	return printData(map[string]any{
		"account":      accountAlias,
		"media_id":     mediaID,
		"output":       cleanPath,
		"bytes":        len(res.Content),
		"content_type": res.ContentType,
		"filename":     res.FileName,
	})
}
