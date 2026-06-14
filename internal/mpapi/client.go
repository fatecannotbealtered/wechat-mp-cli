package mpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const defaultTimeout = 30 * time.Second

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
}

type APIError struct {
	StatusCode int
	ErrCode    int    `json:"errcode,omitempty"`
	ErrMsg     string `json:"errmsg,omitempty"`
}

func (e *APIError) Error() string {
	if e.ErrCode != 0 || e.ErrMsg != "" {
		return fmt.Sprintf("WeChat API error %d: %s", e.ErrCode, e.ErrMsg)
	}
	return fmt.Sprintf("WeChat API HTTP error %d", e.StatusCode)
}

type AccessTokenResponse struct {
	AccessToken string `json:"access_token,omitempty"`
	ExpiresIn   int    `json:"expires_in,omitempty"`
	ErrCode     int    `json:"errcode,omitempty"`
	ErrMsg      string `json:"errmsg,omitempty"`
}

type UploadResponse struct {
	Type      string `json:"type,omitempty"`
	MediaID   string `json:"media_id,omitempty"`
	URL       string `json:"url,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
	ErrCode   int    `json:"errcode,omitempty"`
	ErrMsg    string `json:"errmsg,omitempty"`
}

type MediaDownloadResponse struct {
	JSON        map[string]any
	Content     []byte
	ContentType string
	FileName    string
}

type Article struct {
	Title              string   `json:"title"`
	Author             string   `json:"author,omitempty"`
	Digest             string   `json:"digest,omitempty"`
	Content            string   `json:"content"`
	ContentSourceURL   string   `json:"content_source_url,omitempty"`
	ThumbMediaID       string   `json:"thumb_media_id"`
	NeedOpenComment    *int     `json:"need_open_comment,omitempty"`
	OnlyFansCanComment *int     `json:"only_fans_can_comment,omitempty"`
	ArticleType        string   `json:"article_type,omitempty"`
	ImageMediaIDs      []string `json:"image_info,omitempty"`
}

type DraftAddRequest struct {
	Articles []Article `json:"articles"`
}

type DraftAddResponse struct {
	MediaID string `json:"media_id,omitempty"`
	ErrCode int    `json:"errcode,omitempty"`
	ErrMsg  string `json:"errmsg,omitempty"`
}

type PublishSubmitResponse struct {
	PublishID string `json:"publish_id,omitempty"`
	MsgDataID int64  `json:"msg_data_id,omitempty"`
	ErrCode   int    `json:"errcode,omitempty"`
	ErrMsg    string `json:"errmsg,omitempty"`
}

type MaterialCountResponse struct {
	VoiceCount int    `json:"voice_count"`
	VideoCount int    `json:"video_count"`
	ImageCount int    `json:"image_count"`
	NewsCount  int    `json:"news_count"`
	ErrCode    int    `json:"errcode,omitempty"`
	ErrMsg     string `json:"errmsg,omitempty"`
}

func New(baseURL string) *Client {
	client, err := NewWithProxy(baseURL, "")
	if err != nil {
		return client
	}
	return client
}

func NewWithProxy(baseURL, proxyURL string) (*Client, error) {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.weixin.qq.com"
	}
	httpClient, err := newHTTPClient(proxyURL)
	if err != nil {
		return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTPClient: defaultHTTPClient(), UserAgent: "wechat-mp-cli"}, err
	}
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: httpClient,
		UserAgent:  "wechat-mp-cli",
	}, nil
}

func defaultHTTPClient() *http.Client {
	return &http.Client{
		Timeout: defaultTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			if len(via) > 0 {
				prev := via[len(via)-1]
				if strings.EqualFold(prev.URL.Host, req.URL.Host) {
					req.Header.Set("User-Agent", prev.Header.Get("User-Agent"))
				}
			}
			return nil
		},
	}
}

func newHTTPClient(proxyURL string) (*http.Client, error) {
	client := defaultHTTPClient()
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return client, nil
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		transport.Proxy = http.ProxyURL(parsed)
	case "socks5", "socks5h":
		dialer, err := proxy.FromURL(parsed, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("invalid socks proxy: %w", err)
		}
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			type result struct {
				conn net.Conn
				err  error
			}
			ch := make(chan result, 1)
			go func() {
				conn, err := dialer.Dial(network, addr)
				ch <- result{conn: conn, err: err}
			}()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case res := <-ch:
				return res.conn, res.err
			}
		}
	default:
		return nil, fmt.Errorf("unsupported proxy scheme %q", parsed.Scheme)
	}
	client.Transport = transport
	return client, nil
}

// StableToken fetches an access token via /cgi-bin/stable_token. Unlike the
// classic /cgi-bin/token endpoint, stable_token does NOT invalidate tokens
// already held by other services on the same AppID, so the CLI can run next
// to a production backend without kicking it offline. forceRefresh requests a
// brand-new token (and does invalidate the previous stable token).
func (c *Client) StableToken(ctx context.Context, appID, appSecret string, forceRefresh bool) (*AccessTokenResponse, error) {
	payload := map[string]any{
		"grant_type": "client_credential",
		"appid":      appID,
		"secret":     appSecret,
	}
	if forceRefresh {
		payload["force_refresh"] = true
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := c.BaseURL + "/cgi-bin/stable_token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	var out AccessTokenResponse
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	if out.AccessToken == "" {
		return nil, &APIError{ErrCode: out.ErrCode, ErrMsg: "missing access_token in response"}
	}
	return &out, nil
}

func (c *Client) UploadImage(ctx context.Context, accessToken, imagePath, uploadType string) (*UploadResponse, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return c.UploadImageBytes(ctx, accessToken, data, filepath.Base(imagePath), "application/octet-stream", uploadType)
}

func (c *Client) UploadImageBytes(ctx context.Context, accessToken string, data []byte, filename, contentType, uploadType string) (*UploadResponse, error) {
	if uploadType == "" {
		uploadType = "body"
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	path := "/cgi-bin/media/uploadimg"
	if uploadType == "material" {
		path = "/cgi-bin/material/add_material"
	}
	values := url.Values{}
	values.Set("type", "image")
	return c.uploadMediaBytes(ctx, accessToken, path, values, data, filename, contentType)
}

func (c *Client) UploadTemporaryMedia(ctx context.Context, accessToken, mediaPath, mediaType string) (*UploadResponse, error) {
	file, err := os.Open(mediaPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return c.UploadTemporaryMediaBytes(ctx, accessToken, data, filepath.Base(mediaPath), "application/octet-stream", mediaType)
}

func (c *Client) UploadTemporaryMediaBytes(ctx context.Context, accessToken string, data []byte, filename, contentType, mediaType string) (*UploadResponse, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	values := url.Values{}
	values.Set("type", mediaType)
	return c.uploadMediaBytes(ctx, accessToken, "/cgi-bin/media/upload", values, data, filename, contentType)
}

func (c *Client) uploadMediaBytes(ctx context.Context, accessToken, path string, values url.Values, data []byte, filename, contentType string) (*UploadResponse, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="media"; filename="%s"`, escapeQuotes(filename)))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	if values == nil {
		values = url.Values{}
	}
	values.Set("access_token", accessToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path+"?"+values.Encode(), &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	var out UploadResponse
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func escapeQuotes(value string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, "\\\"").Replace(value)
}

func (c *Client) AddDraft(ctx context.Context, accessToken string, in DraftAddRequest) (*DraftAddResponse, error) {
	var out DraftAddResponse
	if err := c.postJSON(ctx, accessToken, "/cgi-bin/draft/add", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CountDrafts(ctx context.Context, accessToken string) (map[string]any, error) {
	return c.getJSON(ctx, accessToken, "/cgi-bin/draft/count", nil)
}

func (c *Client) UpdateDraft(ctx context.Context, accessToken, mediaID string, index int, article Article) (map[string]any, error) {
	payload := map[string]any{"media_id": mediaID, "index": index, "articles": article}
	var out map[string]any
	if err := c.postJSON(ctx, accessToken, "/cgi-bin/draft/update", payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) BatchGetDraft(ctx context.Context, accessToken string, offset, count int, noContent bool) (map[string]any, error) {
	payload := map[string]any{"offset": offset, "count": count, "no_content": 0}
	if noContent {
		payload["no_content"] = 1
	}
	var out map[string]any
	if err := c.postJSON(ctx, accessToken, "/cgi-bin/draft/batchget", payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetDraft(ctx context.Context, accessToken, mediaID string) (map[string]any, error) {
	var out map[string]any
	if err := c.postJSON(ctx, accessToken, "/cgi-bin/draft/get", map[string]any{"media_id": mediaID}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) DeleteDraft(ctx context.Context, accessToken, mediaID string) (map[string]any, error) {
	var out map[string]any
	if err := c.postJSON(ctx, accessToken, "/cgi-bin/draft/delete", map[string]any{"media_id": mediaID}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) DraftSwitch(ctx context.Context, accessToken string, checkOnly bool) (map[string]any, error) {
	values := url.Values{}
	if checkOnly {
		values.Set("checkonly", "1")
	}
	var out map[string]any
	if err := c.postQuery(ctx, accessToken, "/cgi-bin/draft/switch", values, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) MaterialCount(ctx context.Context, accessToken string) (*MaterialCountResponse, error) {
	values := url.Values{}
	values.Set("access_token", accessToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/cgi-bin/material/get_materialcount?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	var out MaterialCountResponse
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) BatchGetMaterial(ctx context.Context, accessToken, materialType string, offset, count int) (map[string]any, error) {
	payload := map[string]any{"type": materialType, "offset": offset, "count": count}
	var out map[string]any
	if err := c.postJSON(ctx, accessToken, "/cgi-bin/material/batchget_material", payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetMaterial(ctx context.Context, accessToken, mediaID string) (*MediaDownloadResponse, error) {
	return c.postDownloadJSON(ctx, accessToken, "/cgi-bin/material/get_material", map[string]any{"media_id": mediaID})
}

func (c *Client) DeleteMaterial(ctx context.Context, accessToken, mediaID string) (map[string]any, error) {
	var out map[string]any
	if err := c.postJSON(ctx, accessToken, "/cgi-bin/material/del_material", map[string]any{"media_id": mediaID}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetTemporaryMedia(ctx context.Context, accessToken, mediaID string, hdVoice bool) (*MediaDownloadResponse, error) {
	values := url.Values{}
	values.Set("media_id", mediaID)
	path := "/cgi-bin/media/get"
	if hdVoice {
		path = "/cgi-bin/media/get/jssdk"
	}
	return c.getDownload(ctx, accessToken, path, values)
}

func (c *Client) GetCurrentSelfMenu(ctx context.Context, accessToken string) (map[string]any, error) {
	values := url.Values{}
	values.Set("access_token", accessToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/cgi-bin/get_current_selfmenu_info?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	var out map[string]any
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateMenu(ctx context.Context, accessToken string, menu json.RawMessage) (map[string]any, error) {
	var out map[string]any
	if err := c.postJSONRaw(ctx, accessToken, "/cgi-bin/menu/create", menu, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) DeleteMenu(ctx context.Context, accessToken string) (map[string]any, error) {
	values := url.Values{}
	values.Set("access_token", accessToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/cgi-bin/menu/delete?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	var out map[string]any
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AddConditionalMenu creates a personalized (conditional) menu via
// /cgi-bin/menu/addconditional. The payload carries button[] plus a matchrule
// object, so it is passed through as raw JSON like CreateMenu.
func (c *Client) AddConditionalMenu(ctx context.Context, accessToken string, menu json.RawMessage) (map[string]any, error) {
	var out map[string]any
	if err := c.postJSONRaw(ctx, accessToken, "/cgi-bin/menu/addconditional", menu, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateQRCode creates a QR code ticket via /cgi-bin/qrcode/create. payload
// selects the action (temporary QR_SCENE/QR_STR_SCENE vs permanent
// QR_LIMIT_SCENE/QR_LIMIT_STR_SCENE) and scene id; the response carries the
// ticket, expiry, and the encoded url.
func (c *Client) CreateQRCode(ctx context.Context, accessToken string, payload map[string]any) (map[string]any, error) {
	return c.postMap(ctx, accessToken, "/cgi-bin/qrcode/create", payload)
}

// GetUserInfo fetches a single follower profile via /cgi-bin/user/info. The
// nickname/remark fields it returns are user-controlled and must be tagged
// _untrusted by the caller.
func (c *Client) GetUserInfo(ctx context.Context, accessToken, openID, lang string) (map[string]any, error) {
	values := url.Values{}
	values.Set("openid", openID)
	if lang != "" {
		values.Set("lang", lang)
	}
	return c.getJSON(ctx, accessToken, "/cgi-bin/user/info", values)
}

// ListUsers fetches a page of follower openids via /cgi-bin/user/get. nextOpenID
// is the cursor; an empty value starts from the first follower.
func (c *Client) ListUsers(ctx context.Context, accessToken, nextOpenID string) (map[string]any, error) {
	values := url.Values{}
	if nextOpenID != "" {
		values.Set("next_openid", nextOpenID)
	}
	return c.getJSON(ctx, accessToken, "/cgi-bin/user/get", values)
}

// CreateTag creates a follower tag via /cgi-bin/tags/create. The tag name is
// user-facing content and must be tagged _untrusted by the caller.
func (c *Client) CreateTag(ctx context.Context, accessToken, name string) (map[string]any, error) {
	return c.postMap(ctx, accessToken, "/cgi-bin/tags/create", map[string]any{"tag": map[string]any{"name": name}})
}

// ListTags lists all follower tags via /cgi-bin/tags/get.
func (c *Client) ListTags(ctx context.Context, accessToken string) (map[string]any, error) {
	return c.postMap(ctx, accessToken, "/cgi-bin/tags/get", map[string]any{})
}

// UpdateTag renames a follower tag via /cgi-bin/tags/update.
func (c *Client) UpdateTag(ctx context.Context, accessToken string, id int, name string) (map[string]any, error) {
	return c.postMap(ctx, accessToken, "/cgi-bin/tags/update", map[string]any{"tag": map[string]any{"id": id, "name": name}})
}

// DeleteTag deletes a follower tag via /cgi-bin/tags/delete.
func (c *Client) DeleteTag(ctx context.Context, accessToken string, id int) (map[string]any, error) {
	return c.postMap(ctx, accessToken, "/cgi-bin/tags/delete", map[string]any{"tag": map[string]any{"id": id}})
}

// ListTagMembers pages openids carrying a tag via /cgi-bin/user/tag/get.
func (c *Client) ListTagMembers(ctx context.Context, accessToken string, tagID int, nextOpenID string) (map[string]any, error) {
	payload := map[string]any{"tagid": tagID}
	if nextOpenID != "" {
		payload["next_openid"] = nextOpenID
	}
	return c.postMap(ctx, accessToken, "/cgi-bin/user/tag/get", payload)
}

// BatchTag applies a tag to up to 50 followers via
// /cgi-bin/tags/members/batchtagging.
func (c *Client) BatchTag(ctx context.Context, accessToken string, tagID int, openIDs []string) (map[string]any, error) {
	return c.postMap(ctx, accessToken, "/cgi-bin/tags/members/batchtagging", map[string]any{"tagid": tagID, "openid_list": openIDs})
}

// BatchUntag removes a tag from up to 50 followers via
// /cgi-bin/tags/members/batchuntagging.
func (c *Client) BatchUntag(ctx context.Context, accessToken string, tagID int, openIDs []string) (map[string]any, error) {
	return c.postMap(ctx, accessToken, "/cgi-bin/tags/members/batchuntagging", map[string]any{"tagid": tagID, "openid_list": openIDs})
}

func (c *Client) SubmitPublish(ctx context.Context, accessToken, mediaID string) (*PublishSubmitResponse, error) {
	var out PublishSubmitResponse
	if err := c.postJSON(ctx, accessToken, "/cgi-bin/freepublish/submit", map[string]any{"media_id": mediaID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetPublishStatus(ctx context.Context, accessToken, publishID string) (map[string]any, error) {
	var out map[string]any
	if err := c.postJSON(ctx, accessToken, "/cgi-bin/freepublish/get", map[string]any{"publish_id": publishID}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) BatchGetPublish(ctx context.Context, accessToken string, offset, count int, noContent bool) (map[string]any, error) {
	payload := map[string]any{"offset": offset, "count": count, "no_content": 0}
	if noContent {
		payload["no_content"] = 1
	}
	var out map[string]any
	if err := c.postJSON(ctx, accessToken, "/cgi-bin/freepublish/batchget", payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetPublishedArticle(ctx context.Context, accessToken, articleID string) (map[string]any, error) {
	var out map[string]any
	if err := c.postJSON(ctx, accessToken, "/cgi-bin/freepublish/getarticle", map[string]any{"article_id": articleID}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) DeletePublishedArticle(ctx context.Context, accessToken, articleID string, index int) (map[string]any, error) {
	payload := map[string]any{"article_id": articleID, "index": index}
	var out map[string]any
	if err := c.postJSON(ctx, accessToken, "/cgi-bin/freepublish/delete", payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CommentOpen(ctx context.Context, accessToken string, payload map[string]any) (map[string]any, error) {
	return c.postMap(ctx, accessToken, "/cgi-bin/comment/open", payload)
}

func (c *Client) CommentClose(ctx context.Context, accessToken string, payload map[string]any) (map[string]any, error) {
	return c.postMap(ctx, accessToken, "/cgi-bin/comment/close", payload)
}

func (c *Client) CommentList(ctx context.Context, accessToken string, payload map[string]any) (map[string]any, error) {
	return c.postMap(ctx, accessToken, "/cgi-bin/comment/list", payload)
}

func (c *Client) CommentMarkElect(ctx context.Context, accessToken string, payload map[string]any) (map[string]any, error) {
	return c.postMap(ctx, accessToken, "/cgi-bin/comment/markelect", payload)
}

func (c *Client) CommentUnmarkElect(ctx context.Context, accessToken string, payload map[string]any) (map[string]any, error) {
	return c.postMap(ctx, accessToken, "/cgi-bin/comment/unmarkelect", payload)
}

func (c *Client) CommentDelete(ctx context.Context, accessToken string, payload map[string]any) (map[string]any, error) {
	return c.postMap(ctx, accessToken, "/cgi-bin/comment/delete", payload)
}

func (c *Client) CommentReplyAdd(ctx context.Context, accessToken string, payload map[string]any) (map[string]any, error) {
	return c.postMap(ctx, accessToken, "/cgi-bin/comment/reply/add", payload)
}

func (c *Client) CommentReplyDelete(ctx context.Context, accessToken string, payload map[string]any) (map[string]any, error) {
	return c.postMap(ctx, accessToken, "/cgi-bin/comment/reply/delete", payload)
}

func (c *Client) DataCube(ctx context.Context, accessToken, path, beginDate, endDate string) (map[string]any, error) {
	return c.postMap(ctx, accessToken, path, map[string]any{"begin_date": beginDate, "end_date": endDate})
}

func (c *Client) postJSON(ctx context.Context, accessToken, path string, in any, out any) error {
	data, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return c.postJSONRaw(ctx, accessToken, path, data, out)
}

func (c *Client) postMap(ctx context.Context, accessToken, path string, payload map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := c.postJSON(ctx, accessToken, path, payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) getJSON(ctx context.Context, accessToken, path string, params url.Values) (map[string]any, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("access_token", accessToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	var out map[string]any
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) postQuery(ctx context.Context, accessToken, path string, params url.Values, out any) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("access_token", accessToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	return c.do(req, out)
}

func (c *Client) postJSONRaw(ctx context.Context, accessToken, path string, data []byte, out any) error {
	values := url.Values{}
	values.Set("access_token", accessToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path+"?"+values.Encode(), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	return c.do(req, out)
}

func (c *Client) getDownload(ctx context.Context, accessToken, path string, params url.Values) (*MediaDownloadResponse, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("access_token", accessToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	return c.doDownload(req)
}

func (c *Client) postDownloadJSON(ctx context.Context, accessToken, path string, payload any) (*MediaDownloadResponse, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	values.Set("access_token", accessToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path+"?"+values.Encode(), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	return c.doDownload(req)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, ErrMsg: string(body)}
	}
	if len(body) == 0 {
		return nil
	}
	var probe struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &probe); err == nil && probe.ErrCode != 0 {
		return &APIError{StatusCode: resp.StatusCode, ErrCode: probe.ErrCode, ErrMsg: probe.ErrMsg}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}

func (c *Client) doDownload(req *http.Request) (*MediaDownloadResponse, error) {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{StatusCode: resp.StatusCode, ErrMsg: string(body)}
	}
	if looksLikeJSON(contentType, body) {
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err == nil {
			if code, ok := numericErrCode(payload["errcode"]); ok && code != 0 {
				msg, _ := payload["errmsg"].(string)
				return nil, &APIError{StatusCode: resp.StatusCode, ErrCode: code, ErrMsg: msg}
			}
			return &MediaDownloadResponse{JSON: payload, ContentType: contentType}, nil
		}
	}
	return &MediaDownloadResponse{
		Content:     body,
		ContentType: contentType,
		FileName:    fileNameFromContentDisposition(resp.Header.Get("Content-Disposition")),
	}, nil
}

func looksLikeJSON(contentType string, body []byte) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil && strings.Contains(strings.ToLower(mediaType), "json") {
		return true
	}
	trimmed := strings.TrimSpace(string(body))
	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
}

func numericErrCode(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	default:
		return 0, false
	}
}

func fileNameFromContentDisposition(value string) string {
	if value == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(value)
	if err != nil {
		return ""
	}
	return filepath.Base(params["filename"])
}
