package types

import (
	z "github.com/Oudwins/zog"
	"github.com/liukeshao/echo-template/pkg/errors"
)

// ConvertZogIssues 将 zog 验证错误转换为 ErrorDetail 切片
func ConvertZogIssues(issueMap z.ZogIssueMap) []*errors.ErrorDetail {
	if issueMap == nil {
		return nil
	}

	var errorDetails []*errors.ErrorDetail
	for _, issues := range issueMap {
		for _, issue := range issues {
			errorDetails = append(errorDetails, &errors.ErrorDetail{
				Location: issue.Path,
				Message:  issue.Message,
				Value:    issue.Value,
			})
		}
	}

	return errorDetails
}
