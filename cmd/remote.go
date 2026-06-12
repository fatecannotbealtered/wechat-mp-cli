package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Helpers for remote API egress and IP allowlist workflows",
}

var remoteSSH = struct {
	host                  string
	user                  string
	port                  int
	localPort             int
	identityFile          string
	proxyJump             string
	strictHostKeyChecking string
}{port: 22, localPort: 1080, strictHostKeyChecking: "accept-new"}

var remoteSSHCommandCmd = readCommand(&cobra.Command{
	Use:   "ssh-command",
	Short: "Generate an SSH SOCKS tunnel command for remote WeChat API egress",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := required(remoteSSH.host, "--host"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		target := remoteSSH.host
		if strings.TrimSpace(remoteSSH.user) != "" {
			target = remoteSSH.user + "@" + target
		}
		sshArgs := []string{
			"ssh",
			"-N",
			"-D", fmt.Sprintf("127.0.0.1:%d", remoteSSH.localPort),
			"-p", strconv.Itoa(remoteSSH.port),
			"-o", "ExitOnForwardFailure=yes",
			"-o", "ServerAliveInterval=30",
			"-o", "StrictHostKeyChecking=" + remoteSSH.strictHostKeyChecking,
		}
		if strings.TrimSpace(remoteSSH.identityFile) != "" {
			sshArgs = append(sshArgs, "-i", remoteSSH.identityFile)
		}
		if strings.TrimSpace(remoteSSH.proxyJump) != "" {
			sshArgs = append(sshArgs, "-J", remoteSSH.proxyJump)
		}
		sshArgs = append(sshArgs, target)
		proxyURL := fmt.Sprintf("socks5://127.0.0.1:%d", remoteSSH.localPort)
		return printData(map[string]any{
			"ssh_args":            sshArgs,
			"command":             shellJoin(sshArgs),
			"api_proxy":           proxyURL,
			"env":                 "WECHAT_MP_CLI_API_PROXY=" + proxyURL,
			"setup_proxy_dry_run": "wechat-mp-cli setup proxy set --url " + proxyURL + " --dry-run --compact",
			"notes": []string{
				"Run the SSH command in a separate terminal and keep it open.",
				"WeChat API calls will use the remote server IP after WECHAT_MP_CLI_API_PROXY or setup proxy set is applied.",
			},
		})
	},
}, "remote_ssh_command")

func init() {
	remoteSSHCommandCmd.Flags().StringVar(&remoteSSH.host, "host", "", "SSH host whose outbound IP is allowlisted by WeChat")
	remoteSSHCommandCmd.Flags().StringVar(&remoteSSH.user, "user", "", "SSH username")
	remoteSSHCommandCmd.Flags().IntVar(&remoteSSH.port, "port", 22, "SSH port")
	remoteSSHCommandCmd.Flags().IntVar(&remoteSSH.localPort, "local-port", 1080, "Local SOCKS5 port")
	remoteSSHCommandCmd.Flags().StringVar(&remoteSSH.identityFile, "identity-file", "", "SSH private key path")
	remoteSSHCommandCmd.Flags().StringVar(&remoteSSH.proxyJump, "proxy-jump", "", "SSH ProxyJump target")
	remoteSSHCommandCmd.Flags().StringVar(&remoteSSH.strictHostKeyChecking, "strict-host-key-checking", "accept-new", "SSH StrictHostKeyChecking value")
	remoteCmd.AddCommand(remoteSSHCommandCmd)
	rootCmd.AddCommand(remoteCmd)
}

func shellJoin(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "" {
			quoted = append(quoted, `""`)
			continue
		}
		if strings.ContainsAny(arg, " \t\"'`$&|;<>()") {
			quoted = append(quoted, `"`+strings.ReplaceAll(arg, `"`, `\"`)+`"`)
			continue
		}
		quoted = append(quoted, arg)
	}
	return strings.Join(quoted, " ")
}
