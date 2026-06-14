package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/fatecannotbealtered/wechat-mp-cli/internal/audit"
	"github.com/fatecannotbealtered/wechat-mp-cli/internal/config"
	"github.com/fatecannotbealtered/wechat-mp-cli/internal/mpapi"
	"github.com/fatecannotbealtered/wechat-mp-cli/internal/output"
	"github.com/spf13/cobra"
)

const (
	ExitOK        = 0
	ExitError     = 1
	ExitBadArgs   = 2
	ExitNotFound  = 3
	ExitAuth      = 4
	ExitConfirm   = 5
	ExitConflict  = 6
	ExitRetryable = 7
	ExitTimeout   = 8
)

const (
	formatJSON = "json"
	formatText = "text"
	formatRaw  = "raw"

	toolRiskTier    = "T2"
	toolBlastRadius = "Can create and publish public WeChat Official Account content and mutate account-facing assets with the configured app credential permissions."
)

var ErrSilent = errors.New("")

var version = "dev"

var (
	formatMode  = formatJSON
	jsonAlias   bool
	jsonMode    = true
	compactJSON bool
	quietMode   bool
	dryRun      bool
	confirmFlag string
	forceMode   bool

	lastExit      int
	cmdStart      time.Time
	activeCmd     *cobra.Command
	dangerousMode bool
)

var rootCmd = &cobra.Command{
	Use:           "wechat-mp-cli",
	Short:         "AI-native CLI for WeChat Official Account operations",
	Version:       version,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
			version = info.Main.Version
		}
	}
	rootCmd.Version = version
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().StringVar(&formatMode, "format", formatJSON, "Output format: json|text|raw")
	rootCmd.PersistentFlags().BoolVar(&jsonAlias, "json", false, "Compatibility alias for --format json")
	rootCmd.PersistentFlags().BoolVar(&compactJSON, "compact", false, "Compact JSON output")
	rootCmd.PersistentFlags().BoolVar(&quietMode, "quiet", false, "Suppress auxiliary text output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show write preview without executing")
	rootCmd.PersistentFlags().StringVar(&confirmFlag, "confirm", "", "Confirmation token returned by --dry-run")
	rootCmd.PersistentFlags().BoolVar(&forceMode, "force", false, "Reserved for explicit user-approved emergency bypasses")
	rootCmd.PersistentFlags().BoolVar(&dangerousMode, "dangerous", false, "Enable high/critical risk write commands; required in both dry-run and confirm steps")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		cmdStart = time.Now()
		activeCmd = cmd
		output.DurationMS = func() int64 { return time.Since(cmdStart).Milliseconds() }
		if err := configureOutput(cmd); err != nil {
			return err
		}
		// SEC-SPEC §3 second gate: high/critical writes need --dangerous on
		// top of the confirm token, in both the dry-run and confirm steps.
		if cmd.Annotations["write"] == "true" {
			risk := cmd.Annotations["riskLevel"]
			if (risk == "high" || risk == "critical") && !dangerousMode {
				return fail(ExitConfirm, output.ErrConfirmationRequired,
					cmd.CommandPath()+" is "+risk+" risk and requires --dangerous in both dry-run and confirm steps", false)
			}
		}
		return nil
	}
}

func Execute() error {
	return ExecuteContext(context.Background())
}

func ExecuteContext(ctx context.Context) error {
	lastExit = 0
	activeCmd = nil
	cmdStart = time.Now()
	output.DurationMS = func() int64 { return time.Since(cmdStart).Milliseconds() }
	defer func() {
		// CLI-SPEC §10: write commands leave a redacted audit trail; --quiet
		// cannot disable it, and audit failures never disturb the command.
		if activeCmd != nil && activeCmd.Annotations["write"] == "true" {
			audit.Log(activeCmd.CommandPath(), os.Args[1:], lastExit, time.Since(cmdStart).Milliseconds())
		}
		activeCmd = nil
	}()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		if errors.Is(err, ErrSilent) {
			return err
		}
		return fail(ExitBadArgs, output.ErrUsage, err.Error(), false)
	}
	return nil
}

func LastExitCode() int {
	return lastExit
}

func apiCtx() context.Context {
	if activeCmd != nil && activeCmd.Context() != nil {
		return activeCmd.Context()
	}
	return context.Background()
}

func configureOutput(cmd *cobra.Command) error {
	effective := strings.ToLower(strings.TrimSpace(formatMode))
	if effective == "" {
		effective = formatJSON
	}
	formatChanged := cmd.Root().PersistentFlags().Lookup("format").Changed
	jsonChanged := cmd.Root().PersistentFlags().Lookup("json").Changed
	if jsonChanged && formatChanged && effective != formatJSON {
		return fail(ExitBadArgs, output.ErrUsage, "--json cannot be combined with --format "+effective, false)
	}
	if jsonChanged {
		effective = formatJSON
	}
	switch effective {
	case formatJSON, formatText, formatRaw:
	default:
		return fail(ExitBadArgs, output.ErrUsage, "--format must be one of: json, text, raw", false)
	}
	if compactJSON && effective != formatJSON {
		return fail(ExitBadArgs, output.ErrUsage, "--compact can only be used with --format json", false)
	}
	if effective == formatRaw && cmd.Annotations["raw"] != "true" {
		return fail(ExitBadArgs, output.ErrUsage, cmd.CommandPath()+" does not support --format raw", false)
	}
	formatMode = effective
	jsonMode = effective != formatText
	output.Compact = compactJSON && effective == formatJSON
	output.Quiet = quietMode || effective != formatText
	return nil
}

func printData(data any) error {
	if jsonMode {
		output.PrintJSON(data)
		return nil
	}
	output.Text("%v", data)
	return nil
}

func fail(exitCode int, code, message string, retryable bool) error {
	return failWithDetails(exitCode, code, message, nil, retryable)
}

func failWithDetails(exitCode int, code, message string, details any, retryable bool) error {
	setExitCode(exitCode)
	if jsonMode {
		output.PrintErrorJSON(code, message, details, retryable)
	} else {
		output.Error("%s: %s", code, message)
	}
	return ErrSilent
}

func setExitCode(code int) {
	if code > lastExit {
		lastExit = code
	}
}

func handleError(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *mpapi.APIError
	if errors.As(err, &apiErr) {
		code, exit, retryable, details := classifyAPIError(apiErr)
		return failWithDetails(exit, code, apiErr.Error(), details, retryable)
	}
	return fail(ExitError, output.ErrNetwork, err.Error(), true)
}

// classifyCallError maps any client error (a WeChat APIError or a transport
// error) onto the {code, exit, retryable} taxonomy without printing. Batch loops
// use it to record per-item errors while the command keeps running (§15.5).
func classifyCallError(err error) (code string, exit int, retryable bool, details any) {
	var apiErr *mpapi.APIError
	if errors.As(err, &apiErr) {
		return classifyAPIError(apiErr)
	}
	return output.ErrNetwork, ExitError, true, nil
}

// classifyAPIError maps a WeChat APIError onto the error taxonomy. WeChat returns
// HTTP 200 with a business `errcode` for most failures, so the specific errcode
// cases are checked before the HTTP-status cases. Returns the error code, exit
// code, retryability, and optional details (e.g. an actionable hint).
func classifyAPIError(apiErr *mpapi.APIError) (code string, exit int, retryable bool, details any) {
	// Defaults double as the no-match fallthrough so the initial values are read.
	code = output.ErrValidation
	exit = ExitBadArgs
	switch {
	// IP not in allowlist — the entire remote/proxy feature exists to recover
	// from this, so hand the agent the actionable route, not a bare message.
	case apiErr.ErrCode == 40164:
		code, exit, retryable = output.ErrForbidden, ExitAuth, false
		details = map[string]any{
			"errcode": 40164,
			"hint":    "Caller IP is not in the Official Account IP allowlist. Route requests through an allowlisted egress: 'wechat-mp-cli remote ssh-command ...' or 'wechat-mp-cli setup proxy set ...'.",
		}
	// access_token expired — a refresh fixes it, so this is retryable auth.
	case apiErr.ErrCode == 42001:
		code, exit, retryable = output.ErrAuth, ExitAuth, true
	// bad/expired media_id (40007) or invalid menu (41030) — the referenced
	// resource does not exist.
	case apiErr.ErrCode == 40007 || apiErr.ErrCode == 41030:
		code, exit, retryable = output.ErrNotFound, ExitNotFound, false
	case apiErr.StatusCode == 401 || apiErr.ErrCode == 40001 || apiErr.ErrCode == 40014:
		code, exit, retryable = output.ErrAuth, ExitAuth, false
	case apiErr.StatusCode == 403 || apiErr.ErrCode == 48001:
		code, exit, retryable = output.ErrForbidden, ExitAuth, false
	case apiErr.StatusCode == 404:
		code, exit, retryable = output.ErrNotFound, ExitNotFound, false
	case apiErr.StatusCode == 429 || apiErr.ErrCode == 45009:
		code, exit, retryable = output.ErrRateLimited, ExitRetryable, true
	case apiErr.StatusCode >= 500:
		code, exit, retryable = output.ErrServer, ExitRetryable, true
	}
	return code, exit, retryable, details
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func resolveAccount(alias string) (*config.Config, config.Account, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, config.Account{}, err
	}
	account, err := config.ResolveAccount(cfg, alias)
	return cfg, account, err
}

// tokenExpiryMargin keeps a safety window so a token never expires mid-command.
const tokenExpiryMargin = 5 * time.Minute

func accessToken(alias string) (*config.Config, config.Account, string, error) {
	cfg, account, err := resolveAccount(alias)
	if err != nil {
		return nil, config.Account{}, "", err
	}
	// CLI-SPEC §15.1: reuse the cached token until it nears expiry; one CLI
	// invocation must not mint a token per call (quota + production safety).
	if cached, ok := config.LoadCachedToken(account.AppID); ok && time.Until(cached.ExpiresAt) > tokenExpiryMargin {
		return cfg, account, cached.AccessToken, nil
	}
	client, err := apiClient(cfg)
	if err != nil {
		return nil, config.Account{}, "", err
	}
	token, err := client.StableToken(apiCtx(), account.AppID, account.AppSecret, false)
	if err != nil {
		return nil, config.Account{}, "", err
	}
	expiresAt := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	// Cache failures must not break the command; the next call just refetches.
	_ = config.SaveCachedToken(account.AppID, token.AccessToken, expiresAt)
	return cfg, account, token.AccessToken, nil
}

func apiClient(cfg *config.Config) (*mpapi.Client, error) {
	return mpapi.NewWithProxy(cfg.APIBase, cfg.APIProxy)
}

// markUntrusted lists the subtrees of res that carry external, uncontrolled
// content (SEC-SPEC §2): agents must treat them as data, never instructions.
// Only keys actually present in the response are listed.
func markUntrusted(res map[string]any, fields ...string) map[string]any {
	if res == nil {
		return res
	}
	present := make([]string, 0, len(fields))
	for _, f := range fields {
		if _, ok := res[f]; ok {
			present = append(present, f)
		}
	}
	if len(present) > 0 {
		res["_untrusted"] = present
	}
	return res
}

func required(value, name string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func envOrFlagSecret(secret, secretEnv string) string {
	if strings.TrimSpace(secret) != "" {
		return strings.TrimSpace(secret)
	}
	if strings.TrimSpace(secretEnv) == "" {
		return ""
	}
	return strings.TrimSpace(os.Getenv(strings.TrimSpace(secretEnv)))
}
