package render

import (
	"strings"
	"testing"
)

func TestMarkdownToHTML(t *testing.T) {
	html, title := MarkdownToHTML("# Title\n\nHello [site](https://example.com).\n\n![alt](img.png)")
	if title != "Title" {
		t.Fatalf("title = %q", title)
	}
	for _, want := range []string{"<h1>Title</h1>", "<a href=\"https://example.com\">site</a>", "<img src=\"img.png\" alt=\"alt\">"} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q: %s", want, html)
		}
	}
}

func TestMarkdownToDocumentParsesFrontmatter(t *testing.T) {
	doc, err := MarkdownToDocument(`---
title: Frontmatter Title
author: Alice
summary: Short summary
cover: imgs/cover.png
need_open_comment: 1
---

# Body Title

Text.`)
	if err != nil {
		t.Fatalf("MarkdownToDocument() error = %v", err)
	}
	if doc.Title != "Frontmatter Title" || doc.Author != "Alice" || doc.Digest != "Short summary" {
		t.Fatalf("doc metadata = %#v", doc)
	}
	if doc.Cover != "imgs/cover.png" {
		t.Fatalf("cover = %q", doc.Cover)
	}
	if doc.NeedOpenComment == nil || *doc.NeedOpenComment != 1 {
		t.Fatalf("NeedOpenComment = %#v", doc.NeedOpenComment)
	}
}

func TestResolveHTMLImages(t *testing.T) {
	images := ResolveHTMLImages(`<p><img src="imgs/a.png"><img src="https://example.com/b.png"></p>`, `C:\work\post`)
	if len(images) != 2 {
		t.Fatalf("len(images) = %d", len(images))
	}
	if images[0].Remote || images[0].LocalPath == "" {
		t.Fatalf("local image = %#v", images[0])
	}
	if !images[1].Remote || images[1].LocalPath != "" {
		t.Fatalf("remote image = %#v", images[1])
	}
}
