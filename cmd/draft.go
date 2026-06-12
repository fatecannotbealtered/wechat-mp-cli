package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatecannotbealtered/wechat-mp-cli/internal/images"
	"github.com/fatecannotbealtered/wechat-mp-cli/internal/mpapi"
	"github.com/fatecannotbealtered/wechat-mp-cli/internal/render"
	"github.com/spf13/cobra"
)

var errExactlyOneContentSource = errors.New("exactly one content source is required: --content, --content-file, --markdown, or --html")

var draftCmd = &cobra.Command{
	Use:   "draft",
	Short: "Create and manage WeChat Official Account drafts",
}

type draftInputOptions struct {
	account            string
	title              string
	author             string
	digest             string
	content            string
	contentFile        string
	markdownFile       string
	htmlFile           string
	coverMediaID       string
	coverFile          string
	sourceURL          string
	needOpenComment    int
	onlyFansCanComment int
	uploadImages       bool
}

var draftCreate = draftInputOptions{needOpenComment: -1, onlyFansCanComment: -1, uploadImages: true}

var draftCreateCmd = writeCommand(&cobra.Command{
	Use:   "create",
	Short: "Create a WeChat draft article",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := draftCreate
		resolved, err := resolveDraft(opts)
		if err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		if err := required(resolved.Title, "--title, frontmatter title, or a markdown heading"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		if resolved.CoverMediaID == "" && resolved.CoverFile == "" {
			return fail(ExitBadArgs, "E_VALIDATION", "--cover-media-id, --cover-file, frontmatter cover, or a first local inline image is required", false)
		}

		payload := map[string]any{
			"account":             opts.account,
			"title":               resolved.Title,
			"author":              resolved.Author,
			"digest":              resolved.Digest,
			"source":              resolved.Source,
			"content_hash":        sha256Hex(resolved.Doc.HTML),
			"cover_media_id":      resolved.CoverMediaID,
			"cover_file":          cleanOptionalPath(resolved.CoverFile),
			"content_source_url":  resolved.SourceURL,
			"upload_local_images": opts.uploadImages,
		}
		preview := map[string]any{
			"title":               resolved.Title,
			"author":              resolved.Author,
			"digest":              resolved.Digest,
			"source":              resolved.Source,
			"renderer":            resolved.Doc.Renderer,
			"content_size":        len(resolved.Doc.HTML),
			"content_hash":        sha256Hex(resolved.Doc.HTML),
			"cover_source":        resolved.CoverSource,
			"local_image_count":   len(localImageRefs(resolved.Doc.Images)),
			"upload_local_images": opts.uploadImages,
			"will_call_api":       true,
			"api_operation":       "draft/add",
		}
		ok, err := confirmWrite("draft.create", payload, preview)
		if err != nil || !ok {
			return err
		}

		cfg, account, token, err := accessToken(opts.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		contentHTML := resolved.Doc.HTML
		uploadedImages := []map[string]any{}
		if opts.uploadImages {
			var replacements map[string]string
			replacements, uploadedImages, err = uploadLocalContentImages(client, token, resolved.Doc.Images)
			if err != nil {
				return handleError(err)
			}
			contentHTML = render.ReplaceImageSrc(contentHTML, replacements)
		}
		thumbMediaID := resolved.CoverMediaID
		if thumbMediaID == "" {
			preparedCover, err := images.Prepare(resolved.CoverFile, "material")
			if err != nil {
				return handleError(err)
			}
			coverUpload, err := client.UploadImageBytes(apiCtx(), token, preparedCover.Data, preparedCover.Filename, preparedCover.ContentType, "material")
			if err != nil {
				return handleError(err)
			}
			thumbMediaID = coverUpload.MediaID
		}
		article := mpapi.Article{
			Title:            resolved.Title,
			Author:           resolved.Author,
			Digest:           resolved.Digest,
			Content:          contentHTML,
			ContentSourceURL: resolved.SourceURL,
			ThumbMediaID:     thumbMediaID,
		}
		article.NeedOpenComment = resolved.NeedOpenComment
		article.OnlyFansCanComment = resolved.OnlyFansCanComment
		res, err := client.AddDraft(apiCtx(), token, mpapi.DraftAddRequest{Articles: []mpapi.Article{article}})
		if err != nil {
			return handleError(err)
		}
		return printData(map[string]any{
			"account":  account.Alias,
			"app_id":   account.AppID,
			"media_id": res.MediaID,
			"title":    resolved.Title,
			"images":   uploadedImages,
		})
	},
}, "high", "Creates a draft article in the configured WeChat Official Account.")

var draftUpdate = struct {
	draftInputOptions
	mediaID string
	index   int
}{draftInputOptions: draftInputOptions{needOpenComment: -1, onlyFansCanComment: -1, uploadImages: true}}

var draftUpdateCmd = writeCommand(&cobra.Command{
	Use:   "update",
	Short: "Update one article in a WeChat draft",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := required(draftUpdate.mediaID, "--media-id"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		opts := draftUpdate.draftInputOptions
		resolved, err := resolveDraft(opts)
		if err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		if err := required(resolved.Title, "--title, frontmatter title, or a markdown heading"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		if resolved.CoverMediaID == "" && resolved.CoverFile == "" {
			return fail(ExitBadArgs, "E_VALIDATION", "--cover-media-id, --cover-file, frontmatter cover, or a first local inline image is required", false)
		}
		payload := map[string]any{
			"account":             opts.account,
			"media_id":            draftUpdate.mediaID,
			"index":               draftUpdate.index,
			"title":               resolved.Title,
			"source":              resolved.Source,
			"content_hash":        sha256Hex(resolved.Doc.HTML),
			"cover_media_id":      resolved.CoverMediaID,
			"cover_file":          cleanOptionalPath(resolved.CoverFile),
			"upload_local_images": opts.uploadImages,
		}
		ok, err := confirmWrite("draft.update", payload, map[string]any{
			"media_id":            draftUpdate.mediaID,
			"index":               draftUpdate.index,
			"title":               resolved.Title,
			"source":              resolved.Source,
			"renderer":            resolved.Doc.Renderer,
			"content_size":        len(resolved.Doc.HTML),
			"content_hash":        sha256Hex(resolved.Doc.HTML),
			"cover_source":        resolved.CoverSource,
			"local_image_count":   len(localImageRefs(resolved.Doc.Images)),
			"upload_local_images": opts.uploadImages,
			"will_call_api":       true,
			"api_operation":       "draft/update",
		})
		if err != nil || !ok {
			return err
		}
		cfg, account, token, err := accessToken(opts.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		contentHTML := resolved.Doc.HTML
		uploadedImages := []map[string]any{}
		if opts.uploadImages {
			var replacements map[string]string
			replacements, uploadedImages, err = uploadLocalContentImages(client, token, resolved.Doc.Images)
			if err != nil {
				return handleError(err)
			}
			contentHTML = render.ReplaceImageSrc(contentHTML, replacements)
		}
		thumbMediaID := resolved.CoverMediaID
		if thumbMediaID == "" {
			preparedCover, err := images.Prepare(resolved.CoverFile, "material")
			if err != nil {
				return handleError(err)
			}
			coverUpload, err := client.UploadImageBytes(apiCtx(), token, preparedCover.Data, preparedCover.Filename, preparedCover.ContentType, "material")
			if err != nil {
				return handleError(err)
			}
			thumbMediaID = coverUpload.MediaID
		}
		article := mpapi.Article{
			Title:            resolved.Title,
			Author:           resolved.Author,
			Digest:           resolved.Digest,
			Content:          contentHTML,
			ContentSourceURL: resolved.SourceURL,
			ThumbMediaID:     thumbMediaID,
		}
		article.NeedOpenComment = resolved.NeedOpenComment
		article.OnlyFansCanComment = resolved.OnlyFansCanComment
		res, err := client.UpdateDraft(apiCtx(), token, draftUpdate.mediaID, draftUpdate.index, article)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		res["media_id"] = draftUpdate.mediaID
		res["index"] = draftUpdate.index
		res["images"] = uploadedImages
		return printData(res)
	},
}, "high", "Updates an existing WeChat draft article.")

var draftCountAccount string

var draftCountCmd = readCommand(&cobra.Command{
	Use:   "count",
	Short: "Get total draft count",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, account, token, err := accessToken(draftCountAccount)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.CountDrafts(apiCtx(), token)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		return printData(res)
	},
}, "draft_count")

var draftList = struct {
	account   string
	offset    int
	count     int
	noContent bool
}{count: 20}

var draftListCmd = readCommand(&cobra.Command{
	Use:   "list",
	Short: "List draft articles",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, account, token, err := accessToken(draftList.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.BatchGetDraft(apiCtx(), token, draftList.offset, draftList.count, draftList.noContent)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		return printData(markUntrusted(res, "item"))
	},
}, "draft_list")

var draftGet = struct {
	account string
	mediaID string
}{}

var draftGetCmd = readCommand(&cobra.Command{
	Use:   "get",
	Short: "Get one draft by media_id",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := required(draftGet.mediaID, "--media-id"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		cfg, account, token, err := accessToken(draftGet.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.GetDraft(apiCtx(), token, draftGet.mediaID)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		return printData(markUntrusted(res, "news_item"))
	},
}, "draft")

var draftDelete = struct {
	account string
	mediaID string
}{}

var draftDeleteCmd = writeCommand(&cobra.Command{
	Use:   "delete",
	Short: "Delete one draft by media_id",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := required(draftDelete.mediaID, "--media-id"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		payload := map[string]any{"account": draftDelete.account, "media_id": draftDelete.mediaID}
		ok, err := confirmWrite("draft.delete", payload, payload)
		if err != nil || !ok {
			return err
		}
		cfg, account, token, err := accessToken(draftDelete.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.DeleteDraft(apiCtx(), token, draftDelete.mediaID)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		res["deleted_media_id"] = draftDelete.mediaID
		return printData(res)
	},
}, "high", "Deletes a draft from the configured WeChat Official Account.")

var draftSwitchCmd = &cobra.Command{
	Use:   "switch",
	Short: "Manage the official draft and publish switch",
}

var draftSwitchStatus = struct {
	account string
}{}

var draftSwitchStatusCmd = readCommand(&cobra.Command{
	Use:   "status",
	Short: "Check the draft and publish switch state",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, account, token, err := accessToken(draftSwitchStatus.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.DraftSwitch(apiCtx(), token, true)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		return printData(res)
	},
}, "draft_switch")

var draftSwitchEnable = struct {
	account string
}{}

var draftSwitchEnableCmd = writeCommand(&cobra.Command{
	Use:   "enable",
	Short: "Enable the draft and publish switch",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{"account": draftSwitchEnable.account}
		ok, err := confirmWrite("draft.switch.enable", payload, map[string]any{
			"account":        draftSwitchEnable.account,
			"api_operation":  "draft/switch",
			"irreversible":   true,
			"will_call_api":  true,
			"official_scope": "draft and publish switch",
		})
		if err != nil || !ok {
			return err
		}
		cfg, account, token, err := accessToken(draftSwitchEnable.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.DraftSwitch(apiCtx(), token, false)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		return printData(res)
	},
}, "critical", "Irreversibly enables WeChat's draft and publish switch for the configured account.")

func init() {
	bindDraftInputFlags(draftCreateCmd, &draftCreate)
	bindDraftInputFlags(draftUpdateCmd, &draftUpdate.draftInputOptions)
	draftUpdateCmd.Flags().StringVar(&draftUpdate.mediaID, "media-id", "", "Draft media_id")
	draftUpdateCmd.Flags().IntVar(&draftUpdate.index, "index", 0, "Article index inside the draft")
	draftCountCmd.Flags().StringVar(&draftCountAccount, "account", "", "Account alias; defaults to configured default")

	draftListCmd.Flags().StringVar(&draftList.account, "account", "", "Account alias; defaults to configured default")
	draftListCmd.Flags().IntVar(&draftList.offset, "offset", 0, "Draft list offset")
	draftListCmd.Flags().IntVar(&draftList.count, "count", 20, "Draft list count")
	draftListCmd.Flags().BoolVar(&draftList.noContent, "no-content", true, "Omit article content in draft list")

	draftGetCmd.Flags().StringVar(&draftGet.account, "account", "", "Account alias; defaults to configured default")
	draftGetCmd.Flags().StringVar(&draftGet.mediaID, "media-id", "", "Draft media_id")

	draftDeleteCmd.Flags().StringVar(&draftDelete.account, "account", "", "Account alias; defaults to configured default")
	draftDeleteCmd.Flags().StringVar(&draftDelete.mediaID, "media-id", "", "Draft media_id")

	draftSwitchStatusCmd.Flags().StringVar(&draftSwitchStatus.account, "account", "", "Account alias; defaults to configured default")
	draftSwitchEnableCmd.Flags().StringVar(&draftSwitchEnable.account, "account", "", "Account alias; defaults to configured default")
	draftSwitchCmd.AddCommand(draftSwitchStatusCmd, draftSwitchEnableCmd)

	draftCmd.AddCommand(draftCreateCmd, draftUpdateCmd, draftCountCmd, draftListCmd, draftGetCmd, draftDeleteCmd, draftSwitchCmd)
	rootCmd.AddCommand(draftCmd)
}

func bindDraftInputFlags(cmd *cobra.Command, opts *draftInputOptions) {
	cmd.Flags().StringVar(&opts.account, "account", "", "Account alias; defaults to configured default")
	cmd.Flags().StringVar(&opts.title, "title", "", "Article title")
	cmd.Flags().StringVar(&opts.author, "author", "", "Article author")
	cmd.Flags().StringVar(&opts.digest, "digest", "", "Article digest/summary")
	cmd.Flags().StringVar(&opts.content, "content", "", "Raw HTML content")
	cmd.Flags().StringVar(&opts.contentFile, "content-file", "", "Plain or HTML content file")
	cmd.Flags().StringVar(&opts.markdownFile, "markdown", "", "Markdown file to render with the built-in basic renderer")
	cmd.Flags().StringVar(&opts.htmlFile, "html", "", "HTML file to use as article content")
	cmd.Flags().StringVar(&opts.coverMediaID, "cover-media-id", "", "Existing WeChat material media_id for the cover")
	cmd.Flags().StringVar(&opts.coverFile, "cover-file", "", "Local cover image to upload as material")
	cmd.Flags().StringVar(&opts.sourceURL, "source-url", "", "Original source URL")
	cmd.Flags().IntVar(&opts.needOpenComment, "need-open-comment", -1, "Comment setting: -1 omit, 0 closed, 1 open")
	cmd.Flags().IntVar(&opts.onlyFansCanComment, "only-fans-can-comment", -1, "Comment setting: -1 omit, 0 all users, 1 fans only")
	cmd.Flags().BoolVar(&opts.uploadImages, "upload-images", true, "Upload local inline images and replace img src with WeChat URLs")
}

type resolvedDraft struct {
	Doc                *render.Document
	Source             string
	Title              string
	Author             string
	Digest             string
	SourceURL          string
	CoverMediaID       string
	CoverFile          string
	CoverSource        string
	NeedOpenComment    *int
	OnlyFansCanComment *int
}

func resolveDraft(opts draftInputOptions) (*resolvedDraft, error) {
	doc, source, err := resolveDraftDocument(opts)
	if err != nil {
		return nil, err
	}
	resolved := &resolvedDraft{
		Doc:                doc,
		Source:             source,
		Title:              firstNonEmpty(opts.title, doc.Title),
		Author:             firstNonEmpty(opts.author, doc.Author),
		Digest:             firstNonEmpty(opts.digest, doc.Digest),
		SourceURL:          firstNonEmpty(opts.sourceURL, doc.ContentSourceURL),
		CoverMediaID:       strings.TrimSpace(opts.coverMediaID),
		CoverFile:          resolveMaybeRelative(firstNonEmpty(opts.coverFile, doc.Cover), doc.BaseDir),
		CoverSource:        "local_file_upload",
		NeedOpenComment:    doc.NeedOpenComment,
		OnlyFansCanComment: doc.OnlyFansCanComment,
	}
	if opts.needOpenComment >= 0 {
		resolved.NeedOpenComment = &opts.needOpenComment
	}
	if opts.onlyFansCanComment >= 0 {
		resolved.OnlyFansCanComment = &opts.onlyFansCanComment
	}
	if resolved.CoverMediaID != "" {
		resolved.CoverSource = "existing_media_id"
		resolved.CoverFile = ""
		return resolved, nil
	}
	if resolved.CoverFile != "" {
		return resolved, nil
	}
	if first := firstLocalImage(doc.Images); first != "" {
		resolved.CoverFile = first
		resolved.CoverSource = "first_local_inline_image"
	}
	return resolved, nil
}

func resolveDraftDocument(opts draftInputOptions) (*render.Document, string, error) {
	sources := 0
	for _, value := range []string{opts.content, opts.contentFile, opts.markdownFile, opts.htmlFile} {
		if strings.TrimSpace(value) != "" {
			sources++
		}
	}
	if sources != 1 {
		return nil, "", errExactlyOneContentSource
	}
	switch {
	case strings.TrimSpace(opts.content) != "":
		return render.InlineHTML(opts.content), "inline", nil
	case strings.TrimSpace(opts.markdownFile) != "":
		result, err := render.MarkdownFile(opts.markdownFile)
		if err != nil {
			return nil, "", err
		}
		return result, filepath.Clean(opts.markdownFile), nil
	case strings.TrimSpace(opts.htmlFile) != "":
		result, err := render.HTMLFile(opts.htmlFile)
		if err != nil {
			return nil, "", err
		}
		return result, filepath.Clean(opts.htmlFile), nil
	default:
		data, err := os.ReadFile(opts.contentFile)
		if err != nil {
			return nil, "", err
		}
		doc := render.InlineHTML(string(data))
		doc.SourcePath = filepath.Clean(opts.contentFile)
		doc.BaseDir = filepath.Dir(opts.contentFile)
		doc.Images = render.ResolveHTMLImages(doc.HTML, doc.BaseDir)
		return doc, filepath.Clean(opts.contentFile), nil
	}
}

func cleanOptionalPath(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return filepath.Clean(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func resolveMaybeRelative(value, baseDir string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if filepath.IsAbs(value) || baseDir == "" {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(baseDir, value))
}

func firstLocalImage(refs []render.ImageReference) string {
	for _, ref := range refs {
		if ref.LocalPath != "" {
			return ref.LocalPath
		}
	}
	return ""
}

func localImageRefs(refs []render.ImageReference) []render.ImageReference {
	out := []render.ImageReference{}
	for _, ref := range refs {
		if ref.LocalPath != "" {
			out = append(out, ref)
		}
	}
	return out
}

func uploadLocalContentImages(client *mpapi.Client, token string, refs []render.ImageReference) (map[string]string, []map[string]any, error) {
	replacements := map[string]string{}
	uploaded := []map[string]any{}
	seen := map[string]string{}
	for _, ref := range refs {
		if ref.LocalPath == "" {
			continue
		}
		if existing, ok := seen[ref.LocalPath]; ok {
			replacements[ref.Src] = existing
			continue
		}
		prepared, err := images.Prepare(ref.LocalPath, "body")
		if err != nil {
			return nil, nil, err
		}
		res, err := client.UploadImageBytes(apiCtx(), token, prepared.Data, prepared.Filename, prepared.ContentType, "body")
		if err != nil {
			return nil, nil, err
		}
		if strings.TrimSpace(res.URL) == "" {
			return nil, nil, errors.New("body image upload did not return url")
		}
		seen[ref.LocalPath] = res.URL
		replacements[ref.Src] = res.URL
		uploaded = append(uploaded, map[string]any{
			"src":       ref.Src,
			"localPath": ref.LocalPath,
			"url":       res.URL,
			"image":     prepared,
		})
	}
	return replacements, uploaded, nil
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
