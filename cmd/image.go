package cmd

import (
	"path/filepath"
	"strings"

	"github.com/fatecannotbealtered/wechat-mp-cli/internal/images"
	"github.com/spf13/cobra"
)

var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Prepare and upload images for WeChat articles",
}

var imagePrepareCmd = readCommand(&cobra.Command{
	Use:   "prepare <file>",
	Short: "Inspect a local image and report WeChat upload hints",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		info, err := images.Inspect(args[0])
		if err != nil {
			return handleError(err)
		}
		return printData(info)
	},
}, "image_info")

var imageUpload = struct {
	account    string
	uploadType string
}{}

var imageUploadCmd = writeCommand(&cobra.Command{
	Use:   "upload <file>",
	Short: "Upload an image to WeChat as a body image or material image",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uploadType := strings.ToLower(strings.TrimSpace(imageUpload.uploadType))
		if uploadType == "" {
			uploadType = "body"
		}
		if uploadType != "body" && uploadType != "material" {
			return fail(ExitBadArgs, "E_VALIDATION", "--type must be body or material", false)
		}
		payload := map[string]any{
			"account": imageUpload.account,
			"path":    filepath.Clean(args[0]),
			"type":    uploadType,
		}
		ok, err := confirmWrite("image.upload", payload, payload)
		if err != nil || !ok {
			return err
		}
		cfg, account, token, err := accessToken(imageUpload.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		prepared, err := images.Prepare(args[0], uploadType)
		if err != nil {
			return handleError(err)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.UploadImageBytes(apiCtx(), token, prepared.Data, prepared.Filename, prepared.ContentType, uploadType)
		if err != nil {
			return handleError(err)
		}
		return printData(map[string]any{
			"account":  account.Alias,
			"type":     uploadType,
			"media_id": res.MediaID,
			"url":      res.URL,
			"image":    prepared,
		})
	},
}, "medium", "Uploads a local image to the configured WeChat account.")

func init() {
	imageUploadCmd.Flags().StringVar(&imageUpload.account, "account", "", "Account alias; defaults to configured default")
	imageUploadCmd.Flags().StringVar(&imageUpload.uploadType, "type", "body", "Upload type: body|material")
	imageCmd.AddCommand(imagePrepareCmd, imageUploadCmd)
	rootCmd.AddCommand(imageCmd)
}
