package confirm

import "testing"

func TestConfirmTokenBindsPayload(t *testing.T) {
	payload := map[string]any{"media_id": "draft-1"}
	token, _, err := New("publish.submit", payload)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := Verify(token, "publish.submit", payload); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if err := Verify(token, "publish.submit", map[string]any{"media_id": "draft-2"}); err != ErrMismatch {
		t.Fatalf("Verify() mismatch error = %v, want %v", err, ErrMismatch)
	}
}

func TestConfirmTokenBindsOperation(t *testing.T) {
	payload := map[string]any{"alias": "prod"}
	token, _, err := New("setup.account.remove", payload)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := Verify(token, "setup.account.default", payload); err != ErrMismatch {
		t.Fatalf("Verify() operation mismatch error = %v, want %v", err, ErrMismatch)
	}
}
