package codes

import (
	"net/http"

	"github.com/liukeshao/echo-template/pkg/rest"
)

const (
	// ValidationInvalid 表示请求参数未通过字段级校验（与 pkg/rest 保持单一事实来源）。
	ValidationInvalid Code = rest.CodeValidationInvalid
)

func init() {
	Register(ValidationInvalid, http.StatusBadRequest)
}
