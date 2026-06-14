package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manage follower tags",
}

var tagGet = struct {
	account string
}{}

var tagGetCmd = readCommand(&cobra.Command{
	Use:   "get",
	Short: "List all follower tags",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, account, token, err := accessToken(tagGet.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.ListTags(apiCtx(), token)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		// tags[].name is user-defined; tag the subtree as data, not instructions.
		return printData(markUntrusted(res, "tags"))
	},
}, "tag_list")

var tagCreate = struct {
	account string
	name    string
}{}

var tagCreateCmd = writeCommand(&cobra.Command{
	Use:   "create",
	Short: "Create a follower tag",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := required(tagCreate.name, "--name"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		payload := map[string]any{"account": tagCreate.account, "name": tagCreate.name}
		ok, err := confirmWrite("tag.create", payload, map[string]any{
			"account":       tagCreate.account,
			"name":          tagCreate.name,
			"will_call_api": true,
			"api_operation": "tags/create",
		})
		if err != nil || !ok {
			return err
		}
		cfg, account, token, err := accessToken(tagCreate.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.CreateTag(apiCtx(), token, tagCreate.name)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		return printData(markUntrusted(res, "tag"))
	},
}, "medium", "Creates a follower tag for the configured WeChat account.")

var tagUpdate = struct {
	account string
	id      int
	name    string
}{}

var tagUpdateCmd = writeCommand(&cobra.Command{
	Use:   "update",
	Short: "Rename a follower tag",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requirePositiveInt(tagUpdate.id, "--id"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		if err := required(tagUpdate.name, "--name"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		payload := map[string]any{"account": tagUpdate.account, "id": tagUpdate.id, "name": tagUpdate.name}
		ok, err := confirmWrite("tag.update", payload, map[string]any{
			"account":       tagUpdate.account,
			"id":            tagUpdate.id,
			"name":          tagUpdate.name,
			"will_call_api": true,
			"api_operation": "tags/update",
		})
		if err != nil || !ok {
			return err
		}
		cfg, account, token, err := accessToken(tagUpdate.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.UpdateTag(apiCtx(), token, tagUpdate.id, tagUpdate.name)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		return printData(res)
	},
}, "medium", "Renames a follower tag for the configured WeChat account.")

var tagDelete = struct {
	account string
	id      int
}{}

var tagDeleteCmd = writeCommand(&cobra.Command{
	Use:   "delete",
	Short: "Delete a follower tag",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requirePositiveInt(tagDelete.id, "--id"); err != nil {
			return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
		}
		payload := map[string]any{"account": tagDelete.account, "id": tagDelete.id}
		ok, err := confirmWrite("tag.delete", payload, map[string]any{
			"account":       tagDelete.account,
			"id":            tagDelete.id,
			"will_call_api": true,
			"api_operation": "tags/delete",
		})
		if err != nil || !ok {
			return err
		}
		cfg, account, token, err := accessToken(tagDelete.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.DeleteTag(apiCtx(), token, tagDelete.id)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		res["deleted_tag_id"] = tagDelete.id
		return printData(res)
	},
}, "high", "Deletes a follower tag from the configured WeChat account.")

var tagMembers = struct {
	account    string
	nextOpenID string
}{}

var tagMembersCmd = readCommand(&cobra.Command{
	Use:   "members <tagid>",
	Short: "List openids carrying a tag (cursor: next_openid)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tagID, err := strconv.Atoi(strings.TrimSpace(args[0]))
		if err != nil || tagID <= 0 {
			return fail(ExitBadArgs, "E_VALIDATION", "<tagid> must be a positive integer", false)
		}
		cfg, account, token, err := accessToken(tagMembers.account)
		if err != nil {
			return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
		}
		client, err := apiClient(cfg)
		if err != nil {
			return handleError(err)
		}
		res, err := client.ListTagMembers(apiCtx(), token, tagID, tagMembers.nextOpenID)
		if err != nil {
			return handleError(err)
		}
		res["account"] = account.Alias
		res["tagid"] = tagID
		return printData(res)
	},
}, "tag_members")

var tagTagging = struct {
	account string
	id      int
	openIDs []string
}{}

var tagTaggingCmd = writeCommand(&cobra.Command{
	Use:   "tagging",
	Short: "Apply a tag to up to 50 followers",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTagBatch("tag.tagging", "tags/members/batchtagging", tagTagging.account, tagTagging.id, tagTagging.openIDs, true)
	},
}, "medium", "Applies a follower tag to a batch of openids.")

var tagUntagging = struct {
	account string
	id      int
	openIDs []string
}{}

var tagUntaggingCmd = writeCommand(&cobra.Command{
	Use:   "untagging",
	Short: "Remove a tag from up to 50 followers",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTagBatch("tag.untagging", "tags/members/batchuntagging", tagUntagging.account, tagUntagging.id, tagUntagging.openIDs, false)
	},
}, "medium", "Removes a follower tag from a batch of openids.")

func init() {
	tagGetCmd.Flags().StringVar(&tagGet.account, "account", "", "Account alias; defaults to configured default")

	tagCreateCmd.Flags().StringVar(&tagCreate.account, "account", "", "Account alias; defaults to configured default")
	tagCreateCmd.Flags().StringVar(&tagCreate.name, "name", "", "Tag name (max 30 chars)")

	tagUpdateCmd.Flags().StringVar(&tagUpdate.account, "account", "", "Account alias; defaults to configured default")
	tagUpdateCmd.Flags().IntVar(&tagUpdate.id, "id", 0, "Tag id to rename")
	tagUpdateCmd.Flags().StringVar(&tagUpdate.name, "name", "", "New tag name")

	tagDeleteCmd.Flags().StringVar(&tagDelete.account, "account", "", "Account alias; defaults to configured default")
	tagDeleteCmd.Flags().IntVar(&tagDelete.id, "id", 0, "Tag id to delete")

	tagMembersCmd.Flags().StringVar(&tagMembers.account, "account", "", "Account alias; defaults to configured default")
	tagMembersCmd.Flags().StringVar(&tagMembers.nextOpenID, "next-openid", "", "Cursor: start listing after this openid")

	tagTaggingCmd.Flags().StringVar(&tagTagging.account, "account", "", "Account alias; defaults to configured default")
	tagTaggingCmd.Flags().IntVar(&tagTagging.id, "id", 0, "Tag id to apply")
	tagTaggingCmd.Flags().StringSliceVar(&tagTagging.openIDs, "openid", nil, "Follower openid; repeat for up to 50")

	tagUntaggingCmd.Flags().StringVar(&tagUntagging.account, "account", "", "Account alias; defaults to configured default")
	tagUntaggingCmd.Flags().IntVar(&tagUntagging.id, "id", 0, "Tag id to remove")
	tagUntaggingCmd.Flags().StringSliceVar(&tagUntagging.openIDs, "openid", nil, "Follower openid; repeat for up to 50")

	tagCmd.AddCommand(tagGetCmd, tagCreateCmd, tagUpdateCmd, tagDeleteCmd, tagMembersCmd, tagTaggingCmd, tagUntaggingCmd)
	rootCmd.AddCommand(tagCmd)
}

func requirePositiveInt(value int, name string) error {
	if value <= 0 {
		return fmt.Errorf("%s must be greater than 0", name)
	}
	return nil
}

func runTagBatch(operation, apiName, accountAlias string, tagID int, openIDs []string, tagging bool) error {
	if err := requirePositiveInt(tagID, "--id"); err != nil {
		return fail(ExitBadArgs, "E_VALIDATION", err.Error(), false)
	}
	cleaned := make([]string, 0, len(openIDs))
	for _, id := range openIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	if len(cleaned) == 0 {
		return fail(ExitBadArgs, "E_VALIDATION", "--openid is required (repeat for multiple)", false)
	}
	if len(cleaned) > 50 {
		return fail(ExitBadArgs, "E_VALIDATION", "at most 50 --openid values are allowed per call", false)
	}
	payload := map[string]any{"account": accountAlias, "id": tagID, "openid_list": cleaned}
	ok, err := confirmWrite(operation, payload, map[string]any{
		"account":       accountAlias,
		"id":            tagID,
		"openid_count":  len(cleaned),
		"will_call_api": true,
		"api_operation": apiName,
	})
	if err != nil || !ok {
		return err
	}
	cfg, account, token, err := accessToken(accountAlias)
	if err != nil {
		return fail(ExitBadArgs, "E_CONFIG", err.Error(), false)
	}
	client, err := apiClient(cfg)
	if err != nil {
		return handleError(err)
	}
	var res map[string]any
	if tagging {
		res, err = client.BatchTag(apiCtx(), token, tagID, cleaned)
	} else {
		res, err = client.BatchUntag(apiCtx(), token, tagID, cleaned)
	}
	if err != nil {
		return handleError(err)
	}
	res["account"] = account.Alias
	res["tagid"] = tagID
	res["openid_count"] = len(cleaned)
	return printData(res)
}
