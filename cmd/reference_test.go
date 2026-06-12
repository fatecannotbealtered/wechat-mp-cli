package cmd

import "testing"

func TestReferenceMarksPublishSubmitAsConfirmedWrite(t *testing.T) {
	refs := commandRefs(rootCmd)
	publish := findRef(refs, "publish")
	if publish == nil {
		t.Fatal("publish command missing")
	}
	submit := findRef(publish.Commands, "submit")
	if submit == nil {
		t.Fatal("publish submit command missing")
	}
	if submit.Type != "write" || !submit.RequiresConfirmation {
		t.Fatalf("publish submit ref = %#v", submit)
	}
	if submit.RiskLevel != "critical" {
		t.Fatalf("publish submit risk = %q", submit.RiskLevel)
	}
	deleteCmd := findRef(publish.Commands, "delete")
	if deleteCmd == nil {
		t.Fatal("publish delete command missing")
	}
	if deleteCmd.Type != "write" || !deleteCmd.RequiresConfirmation {
		t.Fatalf("publish delete ref = %#v", deleteCmd)
	}
}

func TestReferenceIncludesCoreSelfDescriptionCommands(t *testing.T) {
	refs := commandRefs(rootCmd)
	for _, name := range []string{"context", "doctor", "reference", "changelog", "update"} {
		if findRef(refs, name) == nil {
			t.Fatalf("%s command missing", name)
		}
	}
}

func TestReferenceMarksAssetDeleteAsConfirmedWrite(t *testing.T) {
	asset := findRef(commandRefs(rootCmd), "asset")
	if asset == nil {
		t.Fatal("asset command missing")
	}
	deleteCmd := findRef(asset.Commands, "delete")
	if deleteCmd == nil {
		t.Fatal("asset delete command missing")
	}
	if deleteCmd.Type != "write" || !deleteCmd.RequiresConfirmation {
		t.Fatalf("asset delete ref = %#v", deleteCmd)
	}
}

func TestReferenceMarksMenuSetAsConfirmedWrite(t *testing.T) {
	menu := findRef(commandRefs(rootCmd), "menu")
	if menu == nil {
		t.Fatal("menu command missing")
	}
	setCmd := findRef(menu.Commands, "set")
	if setCmd == nil {
		t.Fatal("menu set command missing")
	}
	if setCmd.Type != "write" || !setCmd.RequiresConfirmation {
		t.Fatalf("menu set ref = %#v", setCmd)
	}
}

func TestReferenceIncludesArticleEndpointGroups(t *testing.T) {
	refs := commandRefs(rootCmd)
	for _, name := range []string{"draft", "publish", "comment", "analytics"} {
		if findRef(refs, name) == nil {
			t.Fatalf("%s command missing", name)
		}
	}
	draft := findRef(refs, "draft")
	for _, name := range []string{"create", "update", "count", "list", "get", "delete", "switch"} {
		if findRef(draft.Commands, name) == nil {
			t.Fatalf("draft %s command missing", name)
		}
	}
	draftSwitch := findRef(draft.Commands, "switch")
	for _, name := range []string{"status", "enable"} {
		if findRef(draftSwitch.Commands, name) == nil {
			t.Fatalf("draft switch %s command missing", name)
		}
	}
	publish := findRef(refs, "publish")
	for _, name := range []string{"submit", "status", "list", "get-article", "delete"} {
		if findRef(publish.Commands, name) == nil {
			t.Fatalf("publish %s command missing", name)
		}
	}
	comment := findRef(refs, "comment")
	for _, name := range []string{"open", "close", "list", "mark", "unmark", "delete", "reply-add", "reply-delete"} {
		if findRef(comment.Commands, name) == nil {
			t.Fatalf("comment %s command missing", name)
		}
	}
	asset := findRef(refs, "asset")
	for _, name := range []string{"count", "list", "get", "delete", "temp"} {
		if findRef(asset.Commands, name) == nil {
			t.Fatalf("asset %s command missing", name)
		}
	}
	temp := findRef(asset.Commands, "temp")
	for _, name := range []string{"upload", "get", "get-hd-voice"} {
		if findRef(temp.Commands, name) == nil {
			t.Fatalf("asset temp %s command missing", name)
		}
	}
	analytics := findRef(refs, "analytics")
	article := findRef(analytics.Commands, "article")
	for _, name := range []string{"published-read", "published-share", "published-summary", "published-detail"} {
		if findRef(article.Commands, name) == nil {
			t.Fatalf("analytics article %s command missing", name)
		}
	}
}

func findRef(refs []refCommand, name string) *refCommand {
	for i := range refs {
		if refs[i].Name == name {
			return &refs[i]
		}
	}
	return nil
}
