package cmd

import "testing"

func TestParsePluralFlagMixedFormsDeDupeOrder(t *testing.T) {
	// Comma-separated and repeated forms mix; duplicates collapse; input order
	// is preserved (CLI-SPEC §15.1).
	got := parsePluralFlag([]string{"a,b", "c", " a ", "d,,b"})
	want := []string{"a", "b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestParsePluralFlagEmpty(t *testing.T) {
	if got := parsePluralFlag([]string{"", " , "}); len(got) != 0 {
		t.Fatalf("blank input should resolve to empty, got %v", got)
	}
}

func TestChunkBoundaries(t *testing.T) {
	in := []string{"1", "2", "3", "4", "5"}
	chunks := chunk(in, 2)
	if len(chunks) != 3 {
		t.Fatalf("want 3 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != 2 || len(chunks[2]) != 1 {
		t.Fatalf("uneven last chunk wrong: %v", chunks)
	}
	// Exact multiple: no trailing empty chunk.
	if got := chunk([]string{"1", "2", "3", "4"}, 2); len(got) != 2 {
		t.Fatalf("exact multiple should give 2 chunks, got %d", len(got))
	}
	// size 0 is a no-op single chunk (defensive).
	if got := chunk(in, 0); len(got) != 1 {
		t.Fatalf("size 0 should give one chunk, got %d", len(got))
	}
}

func TestSummarizeCountsEqualItemTally(t *testing.T) {
	items := []batchItem{
		{Target: "a", OK: true},
		{Target: "b", OK: false, Error: &batchItemErr{Code: "E_NOT_FOUND"}},
		{Target: "c", OK: true},
	}
	s := summarize(items, 4)
	if s.Total != 7 || s.Succeeded != 2 || s.Failed != 1 || s.Skipped != 4 {
		t.Fatalf("summary = %+v", s)
	}
	if s.Succeeded+s.Failed+s.Skipped != s.Total {
		t.Fatalf("counts must sum to total: %+v", s)
	}
}

func TestRequireTargetsCap(t *testing.T) {
	if err := requireTargets(nil, "--openids", 0); err == nil {
		t.Fatal("empty target list should error")
	}
	if err := requireTargets([]string{"a", "b", "c"}, "--openids", 2); err == nil {
		t.Fatal("over-cap list should error")
	}
	if err := requireTargets([]string{"a"}, "--openids", 2); err != nil {
		t.Fatalf("within cap should pass: %v", err)
	}
}

func TestMassSendCapBoundary(t *testing.T) {
	// A mass send is one job and must not silently chunk: at the cap is allowed,
	// one past it is a validation error (CLI-SPEC §15.6 boundary).
	atCap := make([]string, massSendOpenIDCap)
	for i := range atCap {
		atCap[i] = "o" + itoa(i)
	}
	if err := requireTargets(atCap, "--openids", massSendOpenIDCap); err != nil {
		t.Fatalf("exactly at cap should pass: %v", err)
	}
	overCap := make([]string, 0, massSendOpenIDCap+1)
	overCap = append(overCap, atCap...)
	overCap = append(overCap, "oExtra")
	if err := requireTargets(overCap, "--openids", massSendOpenIDCap); err == nil {
		t.Fatal("one past cap should error")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func TestResolveMassMessageExactlyOneBody(t *testing.T) {
	if _, _, err := resolveMassMessage(massBody{}); err == nil {
		t.Fatal("no body should error")
	}
	if _, _, err := resolveMassMessage(massBody{mpnewsMediaID: "m", text: "t"}); err == nil {
		t.Fatal("two bodies should error")
	}
	body, typ, err := resolveMassMessage(massBody{mpnewsMediaID: "MEDIA"})
	if err != nil || typ != "mpnews" || body["msgtype"] != "mpnews" {
		t.Fatalf("mpnews body = %v %q %v", body, typ, err)
	}
	body, typ, err = resolveMassMessage(massBody{text: "hi"})
	if err != nil || typ != "text" || body["msgtype"] != "text" {
		t.Fatalf("text body = %v %q %v", body, typ, err)
	}
}
