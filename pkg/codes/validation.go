package codes

import "net/http"

const (
	// ValidationInvalid 表示请求参数未通过字段级校验。
	ValidationInvalid Code = "VALIDATION_INVALID"
)

func init() {
	Register(ValidationInvalid, http.StatusBadRequest)
}
