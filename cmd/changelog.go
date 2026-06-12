package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var changelogSince string

var changelogCmd = readCommand(&cobra.Command{
	Use:   "changelog",
	Short: "Print changelog entries",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile("CHANGELOG.md")
		if err != nil {
			return printData(map[string]any{
				"since":   changelogSince,
				"entries": []string{"0.1.0: Initial WeChat Official Account CLI scaffold."},
			})
		}
		return printData(map[string]any{
			"since": changelogSince,
			"text":  strings.TrimSpace(string(data)),
		})
	},
}, "changelog")

func init() {
	changelogCmd.Flags().StringVar(&changelogSince, "since", "", "Only show changes since this version when supported")
	rootCmd.AddCommand(changelogCmd)
}
