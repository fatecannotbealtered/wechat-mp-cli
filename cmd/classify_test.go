package cmd

import (
	"strings"
	"testing"

	project "github.com/fatecannotbealtered/wechat-mp-cli"
	"github.com/fatecannotbealtered/wechat-mp-cli/internal/mpapi"
	"github.com/fatecannotbealtered/wechat-mp-cli/internal/output"
)

func TestClassifyAPIError(t *testing.T) {
	tests := []struct {
		name      string
		err       *mpapi.APIError
		wantCode  string
		wantExit  int
		wantRetry bool
		wantHint  bool
	}{
		{"ip_not_allowlisted", &mpapi.APIError{ErrCode: 40164}, output.ErrForbidden, ExitAuth, false, true},
		{"token_expired", &mpapi.APIError{ErrCode: 42001}, output.ErrAuth, ExitAuth, true, false},
		{"bad_media_id", &mpapi.APIError{ErrCode: 40007}, output.ErrNotFound, ExitNotFound, false, false},
		{"invalid_menu", &mpapi.APIError{ErrCode: 41030}, output.ErrNotFound, ExitNotFound, false, false},
		{"invalid_token", &mpapi.APIError{ErrCode: 40001}, output.ErrAuth, ExitAuth, false, false},
		{"rate_limited", &mpapi.APIError{ErrCode: 45009}, output.ErrRateLimited, ExitRetryable, true, false},
		{"server_5xx", &mpapi.APIError{StatusCode: 503}, output.ErrServer, ExitRetryable, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, exit, retryable, details := classifyAPIError(tt.err)
			if code != tt.wantCode || exit != tt.wantExit || retryable != tt.wantRetry {
				t.Fatalf("classifyAPIError(%+v) = (%s,%d,%v), want (%s,%d,%v)",
					tt.err, code, exit, retryable, tt.wantCode, tt.wantExit, tt.wantRetry)
			}
			_, hasHint := details.(map[string]any)
			if hasHint != tt.wantHint {
				t.Fatalf("hint presence = %v, want %v (details=%v)", hasHint, tt.wantHint, details)
			}
			if tt.wantHint {
				m := details.(map[string]any)
				if h, _ := m["hint"].(string); !strings.Contains(h, "remote ssh-command") && !strings.Contains(h, "setup proxy") {
					t.Fatalf("40164 hint should point at remote/proxy recovery, got %q", h)
				}
			}
		})
	}
}

func TestChangelogEmbedded(t *testing.T) {
	if !strings.Contains(project.ChangelogMarkdown, "# Changelog") {
		t.Fatalf("embedded CHANGELOG.md missing expected heading; got first 60 chars: %q",
			project.ChangelogMarkdown[:min(60, len(project.ChangelogMarkdown))])
	}
}
