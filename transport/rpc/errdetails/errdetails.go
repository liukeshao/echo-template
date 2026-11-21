// Package errdetails 定义 google.rpc 错误 detail payload 类型，
// 对应 google.golang.org/genproto/googleapis/rpc/errdetails。
//
// 本包只负责类型定义和 @type URI 常量，不含任何 HTTP 或业务逻辑。
// 参考: https://google.aip.dev/193#status.details
package errdetails

// AIP-193 detail payload 的 @type URI，对应 proto 全限定类型名。
const (
	TypeErrorInfo        = "type.googleapis.com/google.rpc.ErrorInfo"
	TypeLocalizedMessage = "type.googleapis.com/google.rpc.LocalizedMessage"
	TypeBadRequest       = "type.googleapis.com/google.rpc.BadRequest"
	TypeResourceInfo     = "type.googleapis.com/google.rpc.ResourceInfo"
)

// ErrorInfo 是所有错误响应中必须携带的机器可读标识符。
// AIP-193: All error responses must include an ErrorInfo within details.
//
//   - Reason: UPPER_SNAKE_CASE，≤63 字符，匹配 [A-Z][A-Z0-9_]+[A-Z0-9]
//   - Domain: 全局唯一，通常为服务主机名（如 "pubsub.googleapis.com"）
//   - Metadata: 与错误消息中动态内容对应的键值对
type ErrorInfo struct {
	Type     string            `json:"@type"`
	Reason   string            `json:"reason"`
	Domain   string            `json:"domain"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// LocalizedMessage 提供面向用户的本地化错误消息。
// locale 遵循 IETF BCP-47，例如 "en-US"、"zh-CN"。
type LocalizedMessage struct {
	Type    string `json:"@type"`
	Locale  string `json:"locale"`
	Message string `json:"message"`
}

// FieldViolation 描述单个字段的校验失败原因，用于 BadRequest。
type FieldViolation struct {
	Field       string `json:"field"`
	Description string `json:"description"`
}

// BadRequest 描述客户端请求中的字段校验错误。
type BadRequest struct {
	Type            string           `json:"@type"`
	FieldViolations []FieldViolation `json:"fieldViolations,omitempty"`
}

// ResourceInfo 描述访问受限的资源信息，用于 NOT_FOUND 或 ALREADY_EXISTS。
type ResourceInfo struct {
	Type         string `json:"@type"`
	ResourceType string `json:"resourceType"`
	ResourceName string `json:"resourceName"`
	Owner        string `json:"owner,omitempty"`
	Description  string `json:"description,omitempty"`
}
