package render

import (
	"bytes"
	"errors"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

var (
	headingRe = regexp.MustCompile(`(?m)^\s{0,3}#{1,6}\s+(.+?)\s*#*\s*$`)
	imgSrcRe  = regexp.MustCompile(`(?is)<img\b[^>]*\bsrc=["']([^"']+)["'][^>]*>`)
)

type Document struct {
	Title              string            `json:"title,omitempty"`
	Author             string            `json:"author,omitempty"`
	Digest             string            `json:"digest,omitempty"`
	ContentSourceURL   string            `json:"content_source_url,omitempty"`
	Cover              string            `json:"cover,omitempty"`
	NeedOpenComment    *int              `json:"need_open_comment,omitempty"`
	OnlyFansCanComment *int              `json:"only_fans_can_comment,omitempty"`
	HTML               string            `json:"html"`
	ContentSize        int               `json:"content_size"`
	Renderer           string            `json:"renderer"`
	SourcePath         string            `json:"source_path,omitempty"`
	BaseDir            string            `json:"base_dir,omitempty"`
	Images             []ImageReference  `json:"images,omitempty"`
	Frontmatter        map[string]string `json:"frontmatter,omitempty"`
}

type ImageReference struct {
	Src       string `json:"src"`
	LocalPath string `json:"local_path,omitempty"`
	Remote    bool   `json:"remote"`
}

func MarkdownFile(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, err := MarkdownToDocument(string(data))
	if err != nil {
		return nil, err
	}
	doc.SourcePath = filepath.Clean(path)
	doc.BaseDir = filepath.Dir(path)
	doc.Images = ResolveHTMLImages(doc.HTML, doc.BaseDir)
	return doc, nil
}

func HTMLFile(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)
	return &Document{
		Title:       inferTitleFromHTML(content),
		HTML:        content,
		ContentSize: len(content),
		Renderer:    "html-file",
		SourcePath:  filepath.Clean(path),
		BaseDir:     filepath.Dir(path),
		Images:      ResolveHTMLImages(content, filepath.Dir(path)),
	}, nil
}

func InlineHTML(content string) *Document {
	return &Document{
		Title:       inferTitleFromHTML(content),
		HTML:        content,
		ContentSize: len(content),
		Renderer:    "inline-html",
		Images:      ResolveHTMLImages(content, "."),
	}
}

func MarkdownToDocument(markdown string) (*Document, error) {
	frontmatter, body, err := parseFrontmatter(markdown)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
	)
	if err := md.Convert([]byte(body), &buf); err != nil {
		return nil, err
	}
	htmlText := buf.String()
	doc := &Document{
		Title:            firstNonEmpty(frontmatter["title"], extractMarkdownTitle(body), filenameTitle(frontmatter["slug"])),
		Author:           frontmatter["author"],
		Digest:           firstNonEmpty(frontmatter["digest"], frontmatter["summary"], frontmatter["description"], extractSummary(body, 120)),
		ContentSourceURL: firstNonEmpty(frontmatter["contentSourceUrl"], frontmatter["content_source_url"], frontmatter["sourceUrl"], frontmatter["source_url"]),
		Cover:            firstNonEmpty(frontmatter["cover"], frontmatter["coverImage"], frontmatter["cover_image"], frontmatter["featureImage"], frontmatter["feature_image"], frontmatter["image"]),
		HTML:             htmlText,
		ContentSize:      len(htmlText),
		Renderer:         "goldmark-gfm",
		Frontmatter:      frontmatter,
	}
	doc.NeedOpenComment = optionalInt(firstNonEmpty(frontmatter["need_open_comment"], frontmatter["needOpenComment"]))
	doc.OnlyFansCanComment = optionalInt(firstNonEmpty(frontmatter["only_fans_can_comment"], frontmatter["onlyFansCanComment"]))
	return doc, nil
}

func MarkdownToHTML(markdown string) (string, string) {
	doc, err := MarkdownToDocument(markdown)
	if err != nil {
		return "", ""
	}
	return doc.HTML, doc.Title
}

func ResolveHTMLImages(htmlText, baseDir string) []ImageReference {
	seen := map[string]bool{}
	out := []ImageReference{}
	for _, match := range imgSrcRe.FindAllStringSubmatch(htmlText, -1) {
		if len(match) < 2 {
			continue
		}
		src := html.UnescapeString(strings.TrimSpace(match[1]))
		if src == "" || seen[src] {
			continue
		}
		seen[src] = true
		ref := ImageReference{Src: src, Remote: isRemoteImage(src)}
		if !ref.Remote && !strings.HasPrefix(src, "data:") {
			ref.LocalPath = resolveLocalPath(src, baseDir)
		}
		out = append(out, ref)
	}
	return out
}

func ReplaceImageSrc(htmlText string, replacements map[string]string) string {
	if len(replacements) == 0 {
		return htmlText
	}
	return imgSrcRe.ReplaceAllStringFunc(htmlText, func(tag string) string {
		match := imgSrcRe.FindStringSubmatch(tag)
		if len(match) < 2 {
			return tag
		}
		src := html.UnescapeString(strings.TrimSpace(match[1]))
		replacement, ok := replacements[src]
		if !ok {
			return tag
		}
		return strings.Replace(tag, match[1], replacement, 1)
	})
}

func parseFrontmatter(markdown string) (map[string]string, string, error) {
	frontmatter := map[string]string{}
	trimmed := strings.TrimPrefix(markdown, "\ufeff")
	if !strings.HasPrefix(trimmed, "---\n") && !strings.HasPrefix(trimmed, "---\r\n") {
		return frontmatter, trimmed, nil
	}
	normalized := strings.ReplaceAll(trimmed, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return nil, "", errors.New("frontmatter start marker found without closing ---")
	}
	for _, line := range lines[1:end] {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		frontmatter[strings.TrimSpace(key)] = stripQuotes(strings.TrimSpace(value))
	}
	return frontmatter, strings.Join(lines[end+1:], "\n"), nil
}

func extractMarkdownTitle(markdown string) string {
	match := headingRe.FindStringSubmatch(markdown)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(stripMarkdownInline(match[1]))
}

func extractSummary(markdown string, limit int) string {
	for _, block := range strings.Split(markdown, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" || strings.HasPrefix(block, "#") || strings.HasPrefix(block, "```") || strings.HasPrefix(block, "![") {
			continue
		}
		text := strings.Join(strings.Fields(stripMarkdownInline(block)), " ")
		if text == "" {
			continue
		}
		runes := []rune(text)
		if len(runes) > limit {
			return string(runes[:limit])
		}
		return text
	}
	return ""
}

func inferTitleFromHTML(value string) string {
	for _, pattern := range []string{`(?is)<h1[^>]*>(.*?)</h1>`, `(?is)<h2[^>]*>(.*?)</h2>`, `(?is)<title[^>]*>(.*?)</title>`} {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringSubmatch(value); len(match) > 1 {
			return strings.TrimSpace(stripTags(match[1]))
		}
	}
	text := strings.TrimSpace(stripTags(value))
	if len([]rune(text)) > 60 {
		return string([]rune(text)[:60])
	}
	return text
}

func stripMarkdownInline(value string) string {
	value = regexp.MustCompile("!\\[([^\\]]*)\\]\\(([^)]+)\\)").ReplaceAllString(value, "$1")
	value = regexp.MustCompile("\\[([^\\]]+)\\]\\(([^)]+)\\)").ReplaceAllString(value, "$1")
	value = strings.ReplaceAll(value, "`", "")
	value = strings.ReplaceAll(value, "*", "")
	value = strings.ReplaceAll(value, "_", "")
	return value
}

func stripTags(value string) string {
	re := regexp.MustCompile(`(?s)<[^>]+>`)
	return strings.Join(strings.Fields(html.UnescapeString(re.ReplaceAllString(value, " "))), " ")
}

func stripQuotes(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func optionalInt(value string) *int {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	switch strings.ToLower(value) {
	case "true", "yes":
		n := 1
		return &n
	case "false", "no":
		n := 0
		return &n
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &n
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func filenameTitle(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ReplaceAll(value, "-", " ")
}

func isRemoteImage(src string) bool {
	lower := strings.ToLower(src)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func resolveLocalPath(src, baseDir string) string {
	if filepath.IsAbs(src) {
		return filepath.Clean(src)
	}
	cleanSrc := strings.Split(src, "?")[0]
	cleanSrc = strings.Split(cleanSrc, "#")[0]
	return filepath.Clean(filepath.Join(baseDir, cleanSrc))
}
