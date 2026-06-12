package cmd

import (
	"github.com/spf13/cobra"
)

var updateCheck bool

var updateCmd = readCommand(&cobra.Command{
	Use:   "update",
	Short: "Check for CLI updates",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return printData(map[string]any{
			"current_version":  version,
			"check_requested":  updateCheck,
			"update_available": false,
			"message":          "Self-update installation is not implemented in the MVP; use the npm or GitHub release channel.",
		})
	},
}, "update")

func init() {
	updateCmd.Flags().BoolVar(&updateCheck, "check", false, "Check whether a newer version is available")
	rootCmd.AddCommand(updateCmd)
}
