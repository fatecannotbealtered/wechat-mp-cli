package wechatmpcli

import _ "embed"

// ChangelogMarkdown is the CHANGELOG.md embedded into the binary so the
// `changelog` command works regardless of the agent's working directory
// (previously it read ./CHANGELOG.md from the CWD and fell back to a stub).
//
//go:embed CHANGELOG.md
var ChangelogMarkdown string
