package codes

import (
	"fmt"

	rpccodes "github.com/liukeshao/echo-template/transport/rpc/codes"
)

// Domain 是本服务全局唯一的错误域标识。
// AIP-193: 必须稳定，通常为服务主机名。
const Domain = "echo-template"

// oops context key 常量，用于在 oops.With() 和 classifyOops 之间传递 AIP-193 detail payload。
const (
	// CtxKeyMetadata 对应 ErrorInfo.Metadata，值类型为 map[string]string。
	CtxKeyMetadata = "metadata"
	// CtxKeyResourceInfo 对应 ResourceInfo detail，值类型为 errdetails.ResourceInfo。
	CtxKeyResourceInfo = "resource_info"
)

// Code 是机器可读的业务错误码类型。
type Code string

// String 实现 fmt.Stringer，供 oops.Code(c.String()) 使用。
func (c Code) String() string { return string(c) }

// registry 存储业务错误码到 RPC 状态码的映射。
// 各领域在自己文件的 init() 中调用 Register，实现去中心化注册。
var registry = map[Code]rpccodes.Code{}

// Register 注册业务错误码到对应的 RPC 状态码。
// 在各领域文件的 init() 中调用，保证程序启动时完成注册。
// 重复注册同一 Code 时 panic，防止配置错误在运行时静默覆盖。
func Register(c Code, rpcCode rpccodes.Code) {
	if _, exists := registry[c]; exists {
		panic(fmt.Sprintf("codes: duplicate registration for %q", c))
	}
	registry[c] = rpcCode
}

// Lookup 返回业务错误码对应的 RPC 状态码。
// 若未注册则回退到 Internal。
func Lookup(c Code) rpccodes.Code {
	if cc, ok := registry[c]; ok {
		return cc
	}
	return rpccodes.Internal
}

// RegisteredCodes 返回所有已通过 Register 注册的业务错误码副本。
// 顺序不稳定；调用方如需稳定顺序请自行排序。
// 主要供 i18n loader 在启动时校验翻译覆盖率使用。
func RegisteredCodes() []Code {
	out := make([]Code, 0, len(registry))
	for c := range registry {
		out = append(out, c)
	}
	return out
}
