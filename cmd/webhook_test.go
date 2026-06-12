package cmd

import "testing"

func TestWebhookSignatureSortsParts(t *testing.T) {
	got := webhookSignature("token", "timestamp", "nonce")
	want := "6db4861c77e0633e0105672fcd41c9fc2766e26e"
	if got != want {
		t.Fatalf("signature = %q, want %q", got, want)
	}
}
