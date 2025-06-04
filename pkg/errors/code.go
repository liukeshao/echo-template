package errors

// API状态码定义 - 5位数字错误码系统
// 0: 成功
// 1xxxx: 客户端错误
// 2xxxx: 服务器错误
// 3xxxx: 业务错误
const (
	// 成功
	OK = 0

	// 客户端错误 1xxxx
	BadRequest          = 10400 // 请求参数错误
	Unauthorized        = 10401 // 未授权
	Forbidden           = 10403 // 禁止访问
	NotFound            = 10404 // 资源不存在
	MethodNotAllowed    = 10405 // 方法不允许
	Conflict            = 10409 // 资源冲突
	UnprocessableEntity = 10422 // 数据验证失败
	TooManyRequests     = 10429 // 请求过于频繁

	// 服务器错误 2xxxx
	InternalServerError = 20500 // 内部服务器错误
	NotImplemented      = 20501 // 功能未实现
	BadGateway          = 20502 // 网关错误
	ServiceUnavailable  = 20503 // 服务不可用
	GatewayTimeout      = 20504 // 网关超时

	// 业务错误码 3xxxx
	DatabaseError      = 30001 // 数据库错误
	CacheError         = 30002 // 缓存错误
	ExternalAPIError   = 30003 // 外部API错误
	ValidationError    = 30004 // 业务验证错误
	BusinessLogicError = 30005 // 业务逻辑错误
)

// 错误码到消息的映射
var CodeMessages = map[int]string{
	OK: "Success",

	BadRequest:          "Bad Request",
	Unauthorized:        "Unauthorized",
	Forbidden:           "Forbidden",
	NotFound:            "Not Found",
	MethodNotAllowed:    "Method Not Allowed",
	Conflict:            "Conflict",
	UnprocessableEntity: "Unprocessable Entity",
	TooManyRequests:     "Too Many Requests",

	InternalServerError: "Internal Server Error",
	NotImplemented:      "Not Implemented",
	BadGateway:          "Bad Gateway",
	ServiceUnavailable:  "Service Unavailable",
	GatewayTimeout:      "Gateway Timeout",

	DatabaseError:      "Database Error",
	CacheError:         "Cache Error",
	ExternalAPIError:   "External API Error",
	ValidationError:    "Validation Error",
	BusinessLogicError: "Business Logic Error",
}

// GetMessage 根据错误码获取默认消息
func GetMessage(code int) string {
	if msg, ok := CodeMessages[code]; ok {
		return msg
	}
	return "Unknown Error"
}
