package output

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const SchemaVersion = "1.0"

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
)

var (
	Compact    bool
	Quiet      bool
	DurationMS = func() int64 { return 0 }
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

type Meta struct {
	DurationMS int64  `json:"duration_ms"`
	Timestamp  string `json:"timestamp"`
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
	fmt.Fprintf(os.Stdout, format+"\n", args...)
}

func Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func meta() Meta {
	return Meta{
		DurationMS: DurationMS(),
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
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
	fmt.Fprintln(os.Stdout, string(data))
}
