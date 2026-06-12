package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDraftUsesFrontmatterAndFirstImageCover(t *testing.T) {
	resetDraftCreate := draftCreate
	defer func() { draftCreate = resetDraftCreate }()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "imgs"), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	mdPath := filepath.Join(dir, "article.md")
	if err := os.WriteFile(mdPath, []byte(`---
title: FM Title
author: Alice
summary: FM summary
sourceUrl: https://example.com/source
need_open_comment: 1
---

# Ignored Heading

Body text.

![cover](imgs/cover.png)
`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	draftCreate = struct {
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
	}{markdownFile: mdPath, needOpenComment: -1, onlyFansCanComment: -1, uploadImages: true}

	resolved, err := resolveDraft(draftCreate)
	if err != nil {
		t.Fatalf("resolveDraft() error = %v", err)
	}
	if resolved.Title != "FM Title" || resolved.Author != "Alice" || resolved.Digest != "FM summary" {
		t.Fatalf("resolved metadata = %#v", resolved)
	}
	if resolved.SourceURL != "https://example.com/source" {
		t.Fatalf("SourceURL = %q", resolved.SourceURL)
	}
	if resolved.CoverSource != "first_local_inline_image" {
		t.Fatalf("CoverSource = %q", resolved.CoverSource)
	}
	if filepath.Base(resolved.CoverFile) != "cover.png" {
		t.Fatalf("CoverFile = %q", resolved.CoverFile)
	}
	if resolved.NeedOpenComment == nil || *resolved.NeedOpenComment != 1 {
		t.Fatalf("NeedOpenComment = %#v", resolved.NeedOpenComment)
	}
}
