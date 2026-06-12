package cmd

import (
	"strings"
	"testing"
)

func TestShellJoinQuotesSpaces(t *testing.T) {
	got := shellJoin([]string{"ssh", "-i", `C:\Keys\wechat key`, "user@example.com"})
	if !strings.Contains(got, `"C:\Keys\wechat key"`) {
		t.Fatalf("shellJoin() = %q", got)
	}
}
