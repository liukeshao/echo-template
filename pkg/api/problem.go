package api

import "github.com/liukeshao/echo-template/pkg/httpruntime"

// ContentTypeProblemJSON 是 RFC 9457 Problem Details 的标准媒体类型。
const ContentTypeProblemJSON = httpruntime.ContentTypeProblemJSON

// Problem 与 FieldError 重新导出自 pkg/httpruntime，便于调用方仅 import pkg/api 即可访问完整契约类型。
type (
	Problem    = httpruntime.Problem
	FieldError = httpruntime.FieldError
)
