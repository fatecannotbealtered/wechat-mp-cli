package cmd

import (
	"time"

	"github.com/spf13/cobra"
)

var analyticsCmd = &cobra.Command{
	Use:   "analytics",
	Short: "Read WeChat analytics data",
}

var analyticsArticleCmd = &cobra.Command{
	Use:   "article",
	Short: "Read article analytics data",
}

var analyticsUserCmd = &cobra.Command{
	Use:   "user",
	Short: "Read user analytics data",
}

type analyticsOptions struct {
	account   string
	beginDate string
	endDate   string
}

func init() {
	addAnalyticsCommand(analyticsArticleCmd, "summary", "Get daily article summary", "/datacube/getarticlesummary")
	addAnalyticsCommand(analyticsArticleCmd, "total", "Get article total detail", "/datacube/getarticletotal")
	addAnalyticsCommand(analyticsArticleCmd, "read", "Get article read data", "/datacube/getuserread")
	addAnalyticsCommand(analyticsArticleCmd, "read-hour", "Get hourly article read data", "/datacube/getuserreadhour")
	addAnalyticsCommand(analyticsArticleCmd, "share", "Get article share data", "/datacube/getusershare")
	addAnalyticsCommand(analyticsArticleCmd, "share-hour", "Get hourly article share data", "/datacube/getusersharehour")
	addAnalyticsCommand(analyticsArticleCmd, "published-read", "Get published content daily read data", "/datacube/getarticleread")
	addAnalyticsCommand(analyticsArticleCmd, "published-share", "Get published content daily share data", "/datacube/getarticleshare")
	addAnalyticsCommand(analyticsArticleCmd, "published-summary", "Get published content summary data", "/datacube/getbizsummary")
	addAnalyticsCommand(analyticsArticleCmd, "published-detail", "Get published content detail data", "/datacube/getarticletotaldetail")

	addAnalyticsCommand(analyticsUserCmd, "summary", "Get user summary data", "/datacube/getusersummary")
	addAnalyticsCommand(analyticsUserCmd, "cumulate", "Get cumulative user data", "/datacube/getusercumulate")

	analyticsCmd.AddCommand(analyticsArticleCmd, analyticsUserCmd)
	rootCmd.AddCommand(analyticsCmd)
}

func addAnalyticsCommand(parent *cobra.Command, name, short, path string) {
	opts := &analyticsOptions{}
	cmd := readCommand(&cobra.Command{
		Use:   name,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDateRange(opts.beginDate, opts.endDate); err != nil {
				return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
			}
			cfg, account, token, err := accessToken(opts.account)
			if err != nil {
				return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
			}
			client, err := apiClient(cfg)
			if err != nil {
				return handleError(err)
			}
			res, err := client.DataCube(apiCtx(), token, path, opts.beginDate, opts.endDate)
			if err != nil {
				return handleError(err)
			}
			res["account"] = account.Alias
			res["begin_date"] = opts.beginDate
			res["end_date"] = opts.endDate
			return printData(res)
		},
	}, "analytics")
	cmd.Flags().StringVar(&opts.account, "account", "", "Account alias; defaults to configured default")
	cmd.Flags().StringVar(&opts.beginDate, "begin-date", "", "Begin date in YYYY-MM-DD")
	cmd.Flags().StringVar(&opts.endDate, "end-date", "", "End date in YYYY-MM-DD")
	parent.AddCommand(cmd)
}

func validateDateRange(beginDate, endDate string) error {
	if err := required(beginDate, "--begin-date"); err != nil {
		return err
	}
	if err := required(endDate, "--end-date"); err != nil {
		return err
	}
	begin, err := time.Parse("2006-01-02", beginDate)
	if err != nil {
		return err
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return err
	}
	if end.Before(begin) {
		return errEndBeforeBegin
	}
	return nil
}

var errEndBeforeBegin = &dateRangeError{}

type dateRangeError struct{}

func (e *dateRangeError) Error() string {
	return "--end-date must be on or after --begin-date"
}
