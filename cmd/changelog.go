package cmd

import (
	"strings"

	project "github.com/fatecannotbealtered/wechat-mp-cli"
	"github.com/spf13/cobra"
)

var changelogSince string

var changelogCmd = readCommand(&cobra.Command{
	Use:   "changelog",
	Short: "Print changelog entries",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// CHANGELOG.md is embedded into the binary, so this works regardless of
		// the directory the agent runs the binary from.
		return printData(map[string]any{
			"since": changelogSince,
			"text":  strings.TrimSpace(project.ChangelogMarkdown),
		})
	},
}, "changelog")

func init() {
	changelogCmd.Flags().StringVar(&changelogSince, "since", "", "Only show changes since this version when supported")
	rootCmd.AddCommand(changelogCmd)
}
