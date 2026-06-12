package mpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStableTokenSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cgi-bin/stable_token" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if payload["appid"] != "wx123" || payload["secret"] != "secret" {
			t.Fatalf("payload = %#v", payload)
		}
		if _, forced := payload["force_refresh"]; forced {
			t.Fatalf("force_refresh should be omitted when false: %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"token","expires_in":7200}`))
	}))
	defer server.Close()

	client := New(server.URL)
	got, err := client.StableToken(context.Background(), "wx123", "secret", false)
	if err != nil {
		t.Fatalf("StableToken() error = %v", err)
	}
	if got.AccessToken != "token" || got.ExpiresIn != 7200 {
		t.Fatalf("StableToken() = %#v", got)
	}
}

func TestStableTokenForceRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if payload["force_refresh"] != true {
			t.Fatalf("force_refresh missing: %#v", payload)
		}
		w.Write([]byte(`{"access_token":"fresh","expires_in":7200}`))
	}))
	defer server.Close()

	client := New(server.URL)
	got, err := client.StableToken(context.Background(), "wx123", "secret", true)
	if err != nil {
		t.Fatalf("StableToken() error = %v", err)
	}
	if got.AccessToken != "fresh" {
		t.Fatalf("StableToken() = %#v", got)
	}
}

func TestUpdateDraftEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cgi-bin/draft/update" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("access_token") != "token" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if payload["media_id"] != "MEDIA" || payload["index"].(float64) != 1 {
			t.Fatalf("payload = %#v", payload)
		}
		w.Write([]byte(`{"errcode":0}`))
	}))
	defer server.Close()

	client := New(server.URL)
	if _, err := client.UpdateDraft(context.Background(), "token", "MEDIA", 1, Article{Title: "T", Content: "C", ThumbMediaID: "THUMB"}); err != nil {
		t.Fatalf("UpdateDraft() error = %v", err)
	}
}

func TestFreePublishDeleteEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cgi-bin/freepublish/delete" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if payload["article_id"] != "ARTICLE" || payload["index"].(float64) != 0 {
			t.Fatalf("payload = %#v", payload)
		}
		w.Write([]byte(`{"errcode":0}`))
	}))
	defer server.Close()

	client := New(server.URL)
	if _, err := client.DeletePublishedArticle(context.Background(), "token", "ARTICLE", 0); err != nil {
		t.Fatalf("DeletePublishedArticle() error = %v", err)
	}
}

func TestDataCubeEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/datacube/getarticlesummary" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if payload["begin_date"] != "2026-01-01" || payload["end_date"] != "2026-01-02" {
			t.Fatalf("payload = %#v", payload)
		}
		w.Write([]byte(`{"list":[]}`))
	}))
	defer server.Close()

	client := New(server.URL)
	if _, err := client.DataCube(context.Background(), "token", "/datacube/getarticlesummary", "2026-01-01", "2026-01-02"); err != nil {
		t.Fatalf("DataCube() error = %v", err)
	}
}

func TestDraftSwitchStatusEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/cgi-bin/draft/switch" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("access_token") != "token" || r.URL.Query().Get("checkonly") != "1" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if len(body) != 0 {
			t.Fatalf("body = %q", body)
		}
		w.Write([]byte(`{"errcode":0,"errmsg":"ok","is_open":1}`))
	}))
	defer server.Close()

	client := New(server.URL)
	got, err := client.DraftSwitch(context.Background(), "token", true)
	if err != nil {
		t.Fatalf("DraftSwitch() error = %v", err)
	}
	if got["is_open"].(float64) != 1 {
		t.Fatalf("DraftSwitch() = %#v", got)
	}
}

func TestUploadTemporaryMediaEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/cgi-bin/media/upload" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("access_token") != "token" || r.URL.Query().Get("type") != "voice" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader() error = %v", err)
		}
		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("NextPart() error = %v", err)
		}
		if part.FormName() != "media" || part.FileName() != "voice.mp3" {
			t.Fatalf("part = %s %s", part.FormName(), part.FileName())
		}
		w.Write([]byte(`{"type":"voice","media_id":"MEDIA","created_at":1672500000}`))
	}))
	defer server.Close()

	client := New(server.URL)
	got, err := client.UploadTemporaryMediaBytes(context.Background(), "token", []byte("data"), "voice.mp3", "audio/mpeg", "voice")
	if err != nil {
		t.Fatalf("UploadTemporaryMediaBytes() error = %v", err)
	}
	if got.Type != "voice" || got.MediaID != "MEDIA" || got.CreatedAt != 1672500000 {
		t.Fatalf("UploadTemporaryMediaBytes() = %#v", got)
	}
}

func TestGetMaterialJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cgi-bin/material/get_material" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if payload["media_id"] != "MEDIA" {
			t.Fatalf("payload = %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"news_item":[{"title":"T"}]}`))
	}))
	defer server.Close()

	client := New(server.URL)
	got, err := client.GetMaterial(context.Background(), "token", "MEDIA")
	if err != nil {
		t.Fatalf("GetMaterial() error = %v", err)
	}
	if got.JSON == nil || got.JSON["news_item"] == nil {
		t.Fatalf("GetMaterial() = %#v", got)
	}
}

func TestGetTemporaryMediaBinaryResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cgi-bin/media/get" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("media_id") != "MEDIA" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Disposition", `attachment; filename="cover.jpg"`)
		w.Write([]byte("jpeg-data"))
	}))
	defer server.Close()

	client := New(server.URL)
	got, err := client.GetTemporaryMedia(context.Background(), "token", "MEDIA", false)
	if err != nil {
		t.Fatalf("GetTemporaryMedia() error = %v", err)
	}
	if string(got.Content) != "jpeg-data" || got.ContentType != "image/jpeg" || got.FileName != "cover.jpg" {
		t.Fatalf("GetTemporaryMedia() = %#v", got)
	}
}

func TestStableTokenAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"errcode":40013,"errmsg":"invalid appid"}`))
	}))
	defer server.Close()

	client := New(server.URL)
	_, err := client.StableToken(context.Background(), "bad", "secret", false)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want APIError", err)
	}
	if apiErr.ErrCode != 40013 {
		t.Fatalf("ErrCode = %d", apiErr.ErrCode)
	}
}
