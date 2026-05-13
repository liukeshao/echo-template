package codes

import (
	rpccodes "github.com/liukeshao/echo-template/transport/rpc/codes"
)

// Patch 域错误码。供 pkg/patch 在多协议 PATCH 流程中使用：
//
//   - PatchInvalid                : patch 主体无法解析或应用（语法错误、字段不存在等）
//   - PatchUnsupportedMediaType   : Content-Type 不在 SupportedMediaTypes 之列
//
// 二者均映射到 InvalidArgument（HTTP 400）；客户端可通过 ErrorInfo.Reason 区分。
// 之所以不把 PatchUnsupportedMediaType 映射到 Unimplemented（HTTP 501），
// 是因为协议本身（PATCH）是支持的，仅 Content-Type 选项不在白名单内，
// 这属于客户端请求错误而非服务端能力缺失。
const (
	PatchInvalid              Code = "PATCH_INVALID"
	PatchUnsupportedMediaType Code = "PATCH_UNSUPPORTED_MEDIA_TYPE"
)

func init() {
	Register(PatchInvalid, rpccodes.InvalidArgument)
	Register(PatchUnsupportedMediaType, rpccodes.InvalidArgument)
}
