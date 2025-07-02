package types

import (
	"fmt"

	z "github.com/Oudwins/zog"
	"github.com/Oudwins/zog/zconst"
)

type Validator interface {
	Validate() []string
	Shape() z.Shape
}

// FormatIssues 格式化错误信息
func FormatIssues(issueMap z.ZogIssueMap) []string {
	if issueMap == nil {
		return nil
	}

	var errorDetails []string
	for _, issues := range issueMap {
		for _, issue := range issues {
			if issue.Path == zconst.ISSUE_KEY_FIRST || issue.Path == zconst.ISSUE_KEY_ROOT {
				continue
			}
			errorDetails = append(errorDetails, fmt.Sprintf("filed: %s, message: %s", issue.Path, issue.Message))
		}
	}

	return errorDetails
}
