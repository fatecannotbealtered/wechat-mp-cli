---
title: WeChat MP CLI Sample
author: Sean Guo
summary: A small sample article for testing wechat-mp-cli draft creation.
cover: cover.png
sourceUrl: https://example.com/original
need_open_comment: 1
only_fans_can_comment: 0
---

# WeChat MP CLI Sample

This article demonstrates the Markdown frontmatter supported by `wechat-mp-cli`.

Use it for a dry run:

```bash
wechat-mp-cli draft create --markdown examples/article.md --cover-media-id COVER_MEDIA_ID --dry-run --compact
```

The real workflow uploads local inline images after confirmation and replaces their `src` values with WeChat-hosted URLs.
