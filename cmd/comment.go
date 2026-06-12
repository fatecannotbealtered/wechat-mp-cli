package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var commentCmd = &cobra.Command{
	Use:   "comment",
	Short: "Manage article comments",
}

type commentTarget struct {
	account       string
	msgDataID     int64
	index         int
	userCommentID int64
}

var commentList = struct {
	commentTarget
	begin int
	count int
	typ   int
}{count: 50}

var commentListCmd = readCommand(&cobra.Command{
	Use:   "list",
	Short: "List article comments",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requirePositiveInt64(commentList.msgDataID, "--msg-data-id"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		cfg, account, token, err := accessToken(commentList.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.CommentList(apiCtx(), token, map[string]any{
			"msg_data_id": commentList.msgDataID,
			"index":       commentList.index,
			"begin":       commentList.begin,
			"count":       commentList.count,
			"type":        commentList.typ,
		})
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		return printData(markUntrusted(res, "comment"))
	},
}, "comment_list")

var commentOpen commentTarget

var commentOpenCmd = writeCommand(&cobra.Command{
	Use:   "open",
	Short: "Open comments for an article",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCommentSimpleWrite("comment.open", "comment/open", commentOpen, false, nil)
	},
}, "high", "Opens comments for a published article.")

var commentClose commentTarget

var commentCloseCmd = writeCommand(&cobra.Command{
	Use:   "close",
	Short: "Close comments for an article",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCommentSimpleWrite("comment.close", "comment/close", commentClose, false, nil)
	},
}, "high", "Closes comments for a published article.")

var commentMark commentTarget

var commentMarkCmd = writeCommand(&cobra.Command{
	Use:   "mark",
	Short: "Mark a comment as featured",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCommentSimpleWrite("comment.mark", "comment/markelect", commentMark, true, nil)
	},
}, "medium", "Marks a user comment as featured.")

var commentUnmark commentTarget

var commentUnmarkCmd = writeCommand(&cobra.Command{
	Use:   "unmark",
	Short: "Remove featured mark from a comment",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCommentSimpleWrite("comment.unmark", "comment/unmarkelect", commentUnmark, true, nil)
	},
}, "medium", "Removes featured state from a user comment.")

var commentDelete commentTarget

var commentDeleteCmd = writeCommand(&cobra.Command{
	Use:   "delete",
	Short: "Delete a comment",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCommentSimpleWrite("comment.delete", "comment/delete", commentDelete, true, nil)
	},
}, "high", "Deletes a user comment from a published article.")

var commentReplyAdd = struct {
	commentTarget
	content string
}{}

var commentReplyAddCmd = writeCommand(&cobra.Command{
	Use:   "reply-add",
	Short: "Reply to a comment",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := required(commentReplyAdd.content, "--content"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		return runCommentSimpleWrite("comment.reply-add", "comment/reply/add", commentReplyAdd.commentTarget, true, map[string]any{"content": commentReplyAdd.content})
	},
}, "high", "Replies to a user comment as the Official Account.")

var commentReplyDelete commentTarget

var commentReplyDeleteCmd = writeCommand(&cobra.Command{
	Use:   "reply-delete",
	Short: "Delete a reply to a comment",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCommentSimpleWrite("comment.reply-delete", "comment/reply/delete", commentReplyDelete, true, nil)
	},
}, "medium", "Deletes an Official Account reply to a user comment.")

func init() {
	bindCommentTargetFlags(commentListCmd, &commentList.commentTarget, false)
	commentListCmd.Flags().IntVar(&commentList.begin, "begin", 0, "Comment list start offset")
	commentListCmd.Flags().IntVar(&commentList.count, "count", 50, "Comment list count")
	commentListCmd.Flags().IntVar(&commentList.typ, "type", 0, "Comment type filter: 0 all, 1 normal, 2 featured")

	bindCommentTargetFlags(commentOpenCmd, &commentOpen, false)
	bindCommentTargetFlags(commentCloseCmd, &commentClose, false)
	bindCommentTargetFlags(commentMarkCmd, &commentMark, true)
	bindCommentTargetFlags(commentUnmarkCmd, &commentUnmark, true)
	bindCommentTargetFlags(commentDeleteCmd, &commentDelete, true)
	bindCommentTargetFlags(commentReplyAddCmd, &commentReplyAdd.commentTarget, true)
	commentReplyAddCmd.Flags().StringVar(&commentReplyAdd.content, "content", "", "Reply content")
	bindCommentTargetFlags(commentReplyDeleteCmd, &commentReplyDelete, true)

	commentCmd.AddCommand(commentListCmd, commentOpenCmd, commentCloseCmd, commentMarkCmd, commentUnmarkCmd, commentDeleteCmd, commentReplyAddCmd, commentReplyDeleteCmd)
	rootCmd.AddCommand(commentCmd)
}

func bindCommentTargetFlags(cmd *cobra.Command, target *commentTarget, withCommentID bool) {
	cmd.Flags().StringVar(&target.account, "account", "", "Account alias; defaults to configured default")
	cmd.Flags().Int64Var(&target.msgDataID, "msg-data-id", 0, "Published article msg_data_id")
	cmd.Flags().IntVar(&target.index, "index", 0, "Article index inside the publish set")
	if withCommentID {
		cmd.Flags().Int64Var(&target.userCommentID, "user-comment-id", 0, "User comment id")
	}
}

func requirePositiveInt64(value int64, name string) error {
	if value <= 0 {
		return fmt.Errorf("%s must be greater than 0", name)
	}
	return nil
}

func runCommentSimpleWrite(operation, apiName string, target commentTarget, requireCommentID bool, extra map[string]any) error {
	if err := requirePositiveInt64(target.msgDataID, "--msg-data-id"); err != nil {
		return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
	}
	if requireCommentID {
		if err := requirePositiveInt64(target.userCommentID, "--user-comment-id"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
	}
	payload := map[string]any{
		"account":     target.account,
		"msg_data_id": target.msgDataID,
		"index":       target.index,
	}
	if requireCommentID {
		payload["user_comment_id"] = target.userCommentID
	}
	for k, v := range extra {
		payload[k] = v
	}
	ok, err := confirmWrite(operation, payload, map[string]any{
		"payload":       payload,
		"will_call_api": true,
		"api_operation": apiName,
	})
	if err != nil || !ok {
		return err
	}
	cfg, account, token, err := accessToken(target.account)
	if err != nil {
		return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
	}
	client, err := apiClient(cfg)
	if err != nil {
		return handleError(err)
	}
	var res map[string]any
	switch operation {
	case "comment.open":
		res, err = client.CommentOpen(apiCtx(), token, payload)
	case "comment.close":
		res, err = client.CommentClose(apiCtx(), token, payload)
	case "comment.mark":
		res, err = client.CommentMarkElect(apiCtx(), token, payload)
	case "comment.unmark":
		res, err = client.CommentUnmarkElect(apiCtx(), token, payload)
	case "comment.delete":
		res, err = client.CommentDelete(apiCtx(), token, payload)
	case "comment.reply-add":
		res, err = client.CommentReplyAdd(apiCtx(), token, payload)
	case "comment.reply-delete":
		res, err = client.CommentReplyDelete(apiCtx(), token, payload)
	default:
		return fail(ExitBadArgs, "E_VALIDATION", "unknown comment operation", false)
	}
	if err != nil {
		return handleError(err)
	}
	res["account"] = account.Alias
	return printData(res)
}
