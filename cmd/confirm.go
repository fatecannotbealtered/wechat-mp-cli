package cmd

import (
	"errors"

	"github.com/fatecannotbealtered/wechat-mp-cli/internal/confirm"
	"github.com/fatecannotbealtered/wechat-mp-cli/internal/output"
	"github.com/spf13/cobra"
)

type writePreview struct {
	Operation    string `json:"operation"`
	Preview      any    `json:"preview"`
	ConfirmToken string `json:"confirm_token"`
	ExpiresAt    string `json:"expires_at"`
}

func writeCommand(cmd *cobra.Command, risk, blastRadius string) *cobra.Command {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["write"] = "true"
	cmd.Annotations["confirm"] = "true"
	cmd.Annotations["riskLevel"] = risk
	cmd.Annotations["blastRadius"] = blastRadius
	return cmd
}

func readCommand(cmd *cobra.Command, outputType string) *cobra.Command {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["outputType"] = outputType
	return cmd
}

func confirmWrite(operation string, payload any, preview any) (bool, error) {
	if dryRun {
		token, expires, err := confirm.New(operation, payload)
		if err != nil {
			return false, handleError(err)
		}
		return false, printData(writePreview{
			Operation:    operation,
			Preview:      preview,
			ConfirmToken: token,
			ExpiresAt:    expires.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	if forceMode {
		return false, fail(ExitConfirm, output.ErrConfirmationRequired, "--force is reserved; use --dry-run followed by --confirm", false)
	}
	if confirmFlag == "" {
		return false, fail(ExitConfirm, output.ErrConfirmationRequired, "write command requires --dry-run first, then --confirm <confirm_token>", false)
	}
	if err := confirm.Verify(confirmFlag, operation, payload); err != nil {
		msg := "confirm token is invalid for this operation"
		if errors.Is(err, confirm.ErrExpired) {
			msg = "confirm token expired; run --dry-run again"
		}
		return false, fail(ExitConfirm, output.ErrConfirmationRequired, msg, false)
	}
	return true, nil
}
