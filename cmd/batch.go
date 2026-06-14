package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Shared batch contract (CLI-SPEC §15) helpers. A batch command is one
// agent-facing command with one envelope, one confirm token, and one aggregated
// result; the per-item aggregation shape below is identical whether the batch is
// served by a native bulk endpoint (class A) or a client-side loop (class B).

// batchItem is one element of a batch result. target is the natural input key
// (openid, id, …), never an array index, so the agent can zip results back to
// inputs. On failure error carries the same {code, retryable} taxonomy as the
// top-level envelope (§6).
type batchItem struct {
	Target string         `json:"target"`
	OK     bool           `json:"ok"`
	Data   map[string]any `json:"data,omitempty"`
	Error  *batchItemErr  `json:"error,omitempty"`
}

type batchItemErr struct {
	Code      string `json:"code"`
	Retryable bool   `json:"retryable"`
	Message   string `json:"message,omitempty"`
}

// batchSummary always reports total/succeeded/failed; skipped covers the
// unattempted remainder when --continue-on-error=false stops early, so the agent
// can resume.
type batchSummary struct {
	Total     int `json:"total"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	Skipped   int `json:"skipped,omitempty"`
}

// continueOnError is the shared flag default for batch commands: true means
// best-effort (finish the batch); false stops at the first failure. Dangerous
// batches may flip the default — declared per command in reference.
type batchFlags struct {
	continueOnError bool
}

func bindBatchFlags(cmd *cobra.Command, f *batchFlags, defaultContinue bool) {
	f.continueOnError = defaultContinue
	cmd.Flags().BoolVar(&f.continueOnError, "continue-on-error", defaultContinue,
		"Keep processing after an item fails (best-effort); false stops at the first failure")
}

// parsePluralFlag normalizes a repeatable + comma-separated plural flag into a
// de-duplicated, input-order target list (§15.1). Mixed forms
// (--openids a,b --openids c) collapse to one ordered, unique list. An empty
// result is a usage error the caller maps to E_VALIDATION.
func parsePluralFlag(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, entry := range raw {
		for _, part := range strings.Split(entry, ",") {
			v := strings.TrimSpace(part)
			if v == "" {
				continue
			}
			if _, dup := seen[v]; dup {
				continue
			}
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

// chunk splits targets into fixed-size slices so a command can submit a batch
// that exceeds an upstream per-call cap as several sequential calls while
// presenting one command to the agent (§15.6). size must be > 0.
func chunk[T any](items []T, size int) [][]T {
	if size <= 0 {
		return [][]T{items}
	}
	out := make([][]T, 0, (len(items)+size-1)/size)
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		out = append(out, items[i:end])
	}
	return out
}

// summarize tallies an item slice into total/succeeded/failed plus any skipped
// remainder. The counts always equal the item tally (§15.5).
func summarize(items []batchItem, skipped int) batchSummary {
	s := batchSummary{Total: len(items) + skipped, Skipped: skipped}
	for _, it := range items {
		if it.OK {
			s.Succeeded++
		} else {
			s.Failed++
		}
	}
	return s
}

// requireTargets enforces that a resolved plural list is non-empty and within an
// optional hard cap, returning an E_VALIDATION-shaped failure otherwise.
func requireTargets(targets []string, flag string, max int) error {
	if len(targets) == 0 {
		return fmt.Errorf("%s is required (comma-separated or repeated)", flag)
	}
	if max > 0 && len(targets) > max {
		return fmt.Errorf("%s accepts at most %d targets, got %d", flag, max, len(targets))
	}
	return nil
}
