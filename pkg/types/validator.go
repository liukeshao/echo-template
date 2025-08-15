package types

import (
	z "github.com/Oudwins/zog"
	"github.com/Oudwins/zog/zconst"
	"github.com/liukeshao/echo-template/pkg/errors"
)

type Validator interface {
	Validate() *errors.Response
	Shape() z.Shape
}

// FormatIssuesAsErrorDetails 格式化错误信息为ErrorDetail切片
func FormatIssuesAsErrorDetails(issueMap z.ZogIssueMap) []*errors.ErrorDetail {
	if issueMap == nil {
		return nil
	}

	var errorDetails []*errors.ErrorDetail
	for _, issues := range issueMap {
		for _, issue := range issues {
			if issue.Path == zconst.ISSUE_KEY_FIRST || issue.Path == zconst.ISSUE_KEY_ROOT {
				continue
			}
			errorDetails = append(errorDetails, &errors.ErrorDetail{
				Location: issue.Path,
				Message:  issue.Message,
			})
		}
	}

	return errorDetails
}
