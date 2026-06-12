package cmd

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Webhook utilities for WeChat server verification",
}

var webhookVerify = struct {
	token     string
	tokenEnv  string
	timestamp string
	nonce     string
	signature string
	echostr   string
}{}

var webhookVerifyCmd = readCommand(&cobra.Command{
	Use:   "verify",
	Short: "Verify WeChat webhook signature",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		token := strings.TrimSpace(webhookVerify.token)
		if token == "" && strings.TrimSpace(webhookVerify.tokenEnv) != "" {
			token = strings.TrimSpace(os.Getenv(strings.TrimSpace(webhookVerify.tokenEnv)))
		}
		for name, value := range map[string]string{
			"--token or --token-env": token,
			"--timestamp":            webhookVerify.timestamp,
			"--nonce":                webhookVerify.nonce,
			"--signature":            webhookVerify.signature,
		} {
			if err := required(value, name); err != nil {
				return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
			}
		}
		expected := webhookSignature(token, webhookVerify.timestamp, webhookVerify.nonce)
		valid := strings.EqualFold(expected, strings.TrimSpace(webhookVerify.signature))
		return printData(map[string]any{
			"valid":              valid,
			"expected_signature": expected,
			"echostr":            webhookVerify.echostr,
			"echo_allowed":       valid && webhookVerify.echostr != "",
		})
	},
}, "webhook_verify")

func init() {
	webhookVerifyCmd.Flags().StringVar(&webhookVerify.token, "token", "", "Webhook token")
	webhookVerifyCmd.Flags().StringVar(&webhookVerify.tokenEnv, "token-env", "", "Environment variable containing webhook token")
	webhookVerifyCmd.Flags().StringVar(&webhookVerify.timestamp, "timestamp", "", "WeChat timestamp query value")
	webhookVerifyCmd.Flags().StringVar(&webhookVerify.nonce, "nonce", "", "WeChat nonce query value")
	webhookVerifyCmd.Flags().StringVar(&webhookVerify.signature, "signature", "", "WeChat signature query value")
	webhookVerifyCmd.Flags().StringVar(&webhookVerify.echostr, "echostr", "", "Optional echostr query value")
	webhookCmd.AddCommand(webhookVerifyCmd)
	rootCmd.AddCommand(webhookCmd)
}

func webhookSignature(token, timestamp, nonce string) string {
	parts := []string{token, timestamp, nonce}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(sum[:])
}
