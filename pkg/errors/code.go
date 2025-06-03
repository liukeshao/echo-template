package errors

// 标准错误码定义 - 基于HTTP状态码扩展
const (
	// 成功
	OK = 200

	// 客户端错误 4xx
	BadRequest          = 400
	Unauthorized        = 401
	Forbidden           = 403
	NotFound            = 404
	MethodNotAllowed    = 405
	Conflict            = 409
	UnprocessableEntity = 422
	TooManyRequests     = 429

	// 服务器错误 5xx
	InternalServerError = 500
	NotImplemented      = 501
	BadGateway          = 502
	ServiceUnavailable  = 503
	GatewayTimeout      = 504

	// 业务错误码 6xxx
	DatabaseError      = 6001
	CacheError         = 6002
	ExternalAPIError   = 6003
	ValidationError    = 6004
	BusinessLogicError = 6005
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
