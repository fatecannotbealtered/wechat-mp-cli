# 微信官方端点覆盖说明

本文档记录 `wechat-mp-cli` 与微信官方公众号文档的端点对照。检查日期：2026-06-12。

## 来源

- 草稿箱：<https://developers.weixin.qq.com/doc/offiaccount/Draft_Box/Add_draft.html>
- 发布：<https://developers.weixin.qq.com/doc/offiaccount/Publish/Publish.html>
- 留言管理：<https://developers.weixin.qq.com/doc/service/guide/product/comments.html>
- 数据统计：<https://developers.weixin.qq.com/doc/offiaccount/Analytics/Graphic_Analysis_Data_Interface.html>
- 素材管理：<https://developers.weixin.qq.com/doc/offiaccount/Asset_Management/New_temporary_materials.html>

## 已覆盖的文章主链路

| 官方领域 | 端点 | CLI 命令 |
| --- | --- | --- |
| 草稿 | `/cgi-bin/draft/switch`, `/add`, `/update`, `/count`, `/batchget`, `/get`, `/delete` | `draft switch status/enable`, `draft create/update/count/list/get/delete` |
| 发布 | `/cgi-bin/freepublish/submit`, `/get`, `/batchget`, `/getarticle`, `/delete` | `publish submit/status/list/get-article/delete` |
| 留言 | `/cgi-bin/comment/open`, `/close`, `/list`, `/markelect`, `/unmarkelect`, `/delete`, `/reply/add`, `/reply/delete` | `comment open/close/list/mark/unmark/delete/reply-add/reply-delete` |
| 图文/发表内容数据 | `/datacube/getarticlesummary`, `/getarticletotal`, `/getuserread`, `/getuserreadhour`, `/getusershare`, `/getusersharehour`, `/getarticleread`, `/getarticleshare`, `/getbizsummary`, `/getarticletotaldetail` | `analytics article ...` |
| 用户数据 | `/datacube/getusersummary`, `/getusercumulate` | `analytics user summary/cumulate` |
| 文章相关素材 | `/cgi-bin/media/uploadimg`, `/cgi-bin/material/add_material`, `/get_material`, `/get_materialcount`, `/batchget_material`, `/del_material` | `image upload`, `asset count/list/get/delete` |
| 临时素材 | `/cgi-bin/media/upload`, `/get`, `/get/jssdk` | `asset temp upload/get/get-hd-voice` |
| 自定义菜单 | `/cgi-bin/menu/create`, `/delete`, `/get_current_selfmenu_info`, `/addconditional` | `menu set/delete/get/addconditional` |
| 账号二维码 | `/cgi-bin/qrcode/create` | `qrcode create` |
| 粉丝 | `/cgi-bin/user/info`, `/cgi-bin/user/get` | `user info/list` |
| 粉丝标签 | `/cgi-bin/tags/create`, `/get`, `/update`, `/delete`, `/cgi-bin/user/tag/get`, `/cgi-bin/tags/members/batchtagging`, `/batchuntagging` | `tag create/get/update/delete/members/tagging/untagging` |

## 有意保留的边界

本 CLI 不声明覆盖公众号全部 OpenAPI。当前范围是文章生产、发布、评论、素材和文章数据统计主链路，以及相邻的诊断能力。

本轮检查到但暂未实现的数据统计端点：

- 消息数据：`/datacube/getupstreammsg*`。
- 被动回复/接口数据：`/datacube/getinterfacesummary`, `/datacube/getinterfacesummaryhour`。

本轮检查到但暂未实现的草稿相关端点：

- 商品卡片 DOM：`/channels/ec/service/product/getcardinfo`。

这些可以作为后续扩展，但不影响当前草稿、发布、留言、素材和文章统计主链路的完整性。
