package cmd

import "testing"

// TestResolveSendAllFilterXOR pins the audience guard contract: a mass sendall
// must target exactly one of all-followers (--to-all) or a single tag
// (--tag-id). Both-given must fail rather than let --to-all silently win.
func TestResolveSendAllFilterXOR(t *testing.T) {
	tests := []struct {
		name    string
		toAll   bool
		tagID   int
		wantErr bool
		check   func(t *testing.T, filter map[string]any)
	}{
		{
			name:  "only_to_all_ok",
			toAll: true,
			tagID: 0,
			check: func(t *testing.T, filter map[string]any) {
				if filter["is_to_all"] != true {
					t.Fatalf("is_to_all = %v, want true", filter["is_to_all"])
				}
				if _, ok := filter["tag_id"]; ok {
					t.Fatalf("tag_id should be absent for --to-all, got %v", filter["tag_id"])
				}
			},
		},
		{
			name:  "only_tag_ok",
			toAll: false,
			tagID: 7,
			check: func(t *testing.T, filter map[string]any) {
				if filter["is_to_all"] != false {
					t.Fatalf("is_to_all = %v, want false", filter["is_to_all"])
				}
				if filter["tag_id"] != 7 {
					t.Fatalf("tag_id = %v, want 7", filter["tag_id"])
				}
			},
		},
		{
			name:    "both_given_rejected",
			toAll:   true,
			tagID:   7,
			wantErr: true,
		},
		{
			name:    "neither_given_rejected",
			toAll:   false,
			tagID:   0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := resolveSendAllFilter(tt.toAll, tt.tagID)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveSendAllFilter(%v, %d) error = nil, want error", tt.toAll, tt.tagID)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveSendAllFilter(%v, %d) error = %v", tt.toAll, tt.tagID, err)
			}
			tt.check(t, filter)
		})
	}
}
