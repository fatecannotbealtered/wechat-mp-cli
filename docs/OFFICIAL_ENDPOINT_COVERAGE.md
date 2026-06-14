# Official WeChat Endpoint Coverage

This document maps `wechat-mp-cli` commands to the official WeChat Official Account documentation checked on 2026-06-12.

## Sources

- Draft box: <https://developers.weixin.qq.com/doc/offiaccount/Draft_Box/Add_draft.html>
- Publish: <https://developers.weixin.qq.com/doc/offiaccount/Publish/Publish.html>
- Comments: <https://developers.weixin.qq.com/doc/service/guide/product/comments.html>
- Analytics: <https://developers.weixin.qq.com/doc/offiaccount/Analytics/Graphic_Analysis_Data_Interface.html>
- Materials: <https://developers.weixin.qq.com/doc/offiaccount/Asset_Management/New_temporary_materials.html>

## Covered Article Workflow

| Official area | Endpoints | CLI commands |
| --- | --- | --- |
| Drafts | `/cgi-bin/draft/switch`, `/add`, `/update`, `/count`, `/batchget`, `/get`, `/delete` | `draft switch status/enable`, `draft create/update/count/list/get/delete` |
| Publish | `/cgi-bin/freepublish/submit`, `/get`, `/batchget`, `/getarticle`, `/delete` | `publish submit/status/list/get-article/delete` |
| Comments | `/cgi-bin/comment/open`, `/close`, `/list`, `/markelect`, `/unmarkelect`, `/delete`, `/reply/add`, `/reply/delete` | `comment open/close/list/mark/unmark/delete/reply-add/reply-delete` |
| Article analytics | `/datacube/getarticlesummary`, `/getarticletotal`, `/getuserread`, `/getuserreadhour`, `/getusershare`, `/getusersharehour`, `/getarticleread`, `/getarticleshare`, `/getbizsummary`, `/getarticletotaldetail` | `analytics article ...` |
| User analytics | `/datacube/getusersummary`, `/getusercumulate` | `analytics user summary/cumulate` |
| Materials used by articles | `/cgi-bin/media/uploadimg`, `/cgi-bin/material/add_material`, `/get_material`, `/get_materialcount`, `/batchget_material`, `/del_material` | `image upload`, `asset count/list/get/delete` |
| Temporary media | `/cgi-bin/media/upload`, `/get`, `/get/jssdk` | `asset temp upload/get/get-hd-voice` |
| Menu | `/cgi-bin/menu/create`, `/delete`, `/get_current_selfmenu_info`, `/addconditional` | `menu set/delete/get/addconditional` |
| Account QR codes | `/cgi-bin/qrcode/create` | `qrcode create` |
| Followers | `/cgi-bin/user/info`, `/cgi-bin/user/get` | `user info/list` |
| Follower tags | `/cgi-bin/tags/create`, `/get`, `/update`, `/delete`, `/cgi-bin/user/tag/get`, `/cgi-bin/tags/members/batchtagging`, `/batchuntagging` | `tag create/get/update/delete/members/tagging/untagging` |

## Intentional Boundaries

The CLI does not claim to cover every Official Account API. Current scope is the article production and publication workflow plus adjacent diagnostics.

Not yet implemented from the checked analytics page:

- Message analytics: `/datacube/getupstreammsg*`.
- Passive reply/interface analytics: `/datacube/getinterfacesummary`, `/datacube/getinterfacesummaryhour`.

Not yet implemented from the checked draft page:

- Product card DOM endpoint: `/channels/ec/service/product/getcardinfo`.

These are useful future extensions, but they are not required for the core article draft, publish, comment, material, and article analytics path.
