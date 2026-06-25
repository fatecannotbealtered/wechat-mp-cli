package output

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatecannotbealtered/wechat-mp-cli/internal/contract"
)

// SchemaVersion is sourced from the canonical contract (contract/contract.json
// via internal/contract/contract_gen.go), so the JSON schema version cannot drift
// from the fleet contract.
const SchemaVersion = contract.SchemaVersion

const (
	ErrUsage                = "E_USAGE"
	ErrValidation           = "E_VALIDATION"
	ErrNotFound             = "E_NOT_FOUND"
	ErrAuth                 = "E_AUTH"
	ErrForbidden            = "E_FORBIDDEN"
	ErrConfig               = "E_CONFIG"
	ErrConfirmationRequired = "E_CONFIRMATION_REQUIRED"
	ErrConflict             = "E_CONFLICT"
	ErrNetwork              = "E_NETWORK"
	ErrRateLimited          = "E_RATE_LIMITED"
	ErrServer               = "E_SERVER"
	ErrTimeout              = "E_TIMEOUT"
	ErrIntegrity            = "E_INTEGRITY"
	ErrIO                   = "E_IO"
	ErrInterrupted          = "E_INTERRUPTED"
)

var (
	Compact    bool
	Quiet      bool
	DurationMS = func() int64 { return 0 }
	// UpdateNoticesProvider is a func-pointer hook set from package cmd (init).
	// It returns the cached update notice(s) to piggyback on meta.notices,
	// read-only from the local cache with zero network I/O. It lives here as a
	// hook to break the output -> cmd import cycle (same pattern as DurationMS).
	// nil means "no provider wired" and yields no meta.notices.
	UpdateNoticesProvider func() []any
)

type Envelope struct {
	OK            bool           `json:"ok"`
	SchemaVersion string         `json:"schema_version"`
	Data          any            `json:"data,omitempty"`
	Error         *ErrorEnvelope `json:"error,omitempty"`
	Meta          Meta           `json:"meta"`
}

type ErrorEnvelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   any    `json:"details,omitempty"`
	Retryable bool   `json:"retryable"`
}

// Meta carries the envelope metadata. Canonical keys are duration_ms and
// notices (CLI-SPEC §3 / contract.json); no other keys may appear here.
type Meta struct {
	DurationMS int64 `json:"duration_ms"`
	// Notices carries the cached update notice (CLI-SPEC §3 / §14), read-only
	// from the local cache. omitempty: present only when the cache currently
	// holds an available-update notice; absent otherwise.
	Notices []any `json:"notices,omitempty"`
}

// ExitCodeForErrorCode maps a semantic error code to its process exit code.
// The mapping is sourced from the canonical contract (internal/contract), so
// it cannot drift from the fleet's E_* -> exit table (CLI-SPEC §6).
func ExitCodeForErrorCode(code string) int {
	return contract.ExitFor(code)
}

// RetryableForErrorCode reports whether an agent may retry an error code.
// Sourced from the canonical contract (internal/contract) so it cannot drift
// from the fleet's retryability table.
func RetryableForErrorCode(code string) bool {
	return contract.Retryable(code)
}

func PrintJSON(data any) {
	printEnvelope(Envelope{
		OK:            true,
		SchemaVersion: SchemaVersion,
		Data:          data,
		Meta:          meta(),
	})
}

func PrintErrorJSON(code, message string, details any, retryable bool) {
	printEnvelope(Envelope{
		OK:            false,
		SchemaVersion: SchemaVersion,
		Error: &ErrorEnvelope{
			Code:      code,
			Message:   message,
			Details:   details,
			Retryable: retryable,
		},
		Meta: meta(),
	})
}

func Text(format string, args ...any) {
	if Quiet {
		return
	}
	_, _ = fmt.Fprintf(os.Stdout, format+"\n", args...)
}

func Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func meta() Meta {
	m := Meta{
		DurationMS: DurationMS(),
	}
	if UpdateNoticesProvider != nil {
		if notices := UpdateNoticesProvider(); len(notices) > 0 {
			m.Notices = notices
		}
	}
	return m
}

func printEnvelope(env Envelope) {
	var (
		data []byte
		err  error
	)
	if Compact {
		data, err = json.Marshal(env)
	} else {
		data, err = json.MarshalIndent(env, "", "  ")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode JSON output: %v\n", err)
		return
	}
	_, _ = fmt.Fprintln(os.Stdout, string(data))
}
