package codes

import rpccodes "github.com/liukeshao/echo-template/transport/rpc/codes"

const (
	// ValidationInvalid 表示请求参数未通过字段级校验。
	ValidationInvalid Code = "VALIDATION_INVALID"
)

func init() {
	Register(ValidationInvalid, rpccodes.InvalidArgument)
}
