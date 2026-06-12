package cmd

import (
	"github.com/fatecannotbealtered/wechat-mp-cli/internal/render"
	"github.com/spf13/cobra"
)

var renderCmd = &cobra.Command{
	Use:   "render",
	Short: "Render local content into WeChat-ready intermediate formats",
}

var renderMarkdownCmd = readCommand(&cobra.Command{
	Use:   "markdown <file>",
	Short: "Render markdown with the built-in basic renderer",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := render.MarkdownFile(args[0])
		if err != nil {
			return handleError(err)
		}
		return printData(result)
	},
}, "rendered_html")

var renderHTMLCmd = readCommand(&cobra.Command{
	Use:   "html <file>",
	Short: "Load an HTML file and return metadata",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := render.HTMLFile(args[0])
		if err != nil {
			return handleError(err)
		}
		return printData(result)
	},
}, "rendered_html")

func init() {
	renderCmd.AddCommand(renderMarkdownCmd, renderHTMLCmd)
	rootCmd.AddCommand(renderCmd)
}
