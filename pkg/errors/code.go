package errors

// API状态码定义 - 字符串错误码系统
const (
	// 成功
	OK = "ok"

	// 客户端错误 1xxxx
	BadRequest          = "bad_request"          // 请求参数错误
	Unauthorized        = "unauthorized"         // 未授权
	Forbidden           = "forbidden"            // 禁止访问
	NotFound            = "not_found"            // 资源不存在
	MethodNotAllowed    = "method_not_allowed"   // 方法不允许
	Conflict            = "conflict"             // 资源冲突
	UnprocessableEntity = "unprocessable_entity" // 数据验证失败
	TooManyRequests     = "too_many_requests"    // 请求过于频繁

	// 服务器错误 2xxxx
	InternalServerError = "internal_server_error" // 内部服务器错误
	NotImplemented      = "not_implemented"       // 功能未实现
	BadGateway          = "bad_gateway"           // 网关错误
	ServiceUnavailable  = "service_unavailable"   // 服务不可用
	GatewayTimeout      = "gateway_timeout"       // 网关超时

	// 业务错误码 3xxxx
	DatabaseError      = "database_error"       // 数据库错误
	CacheError         = "cache_error"          // 缓存错误
	ExternalAPIError   = "external_api_error"   // 外部API错误
	ValidationError    = "validation_error"     // 业务验证错误
	BusinessLogicError = "business_logic_error" // 业务逻辑错误
)

// 预置的错误构建器
var (
	// ErrDatabase 数据库错误构建器
	ErrDatabase = Code(DatabaseError).Tags("database")

	// ErrBadRequest 请求参数错误构建器
	ErrBadRequest = Code(BadRequest).Tags("client")

	// ErrUnauthorized 未授权错误构建器
	ErrUnauthorized = Code(Unauthorized).Tags("auth")

	// ErrForbidden 禁止访问错误构建器
	ErrForbidden = Code(Forbidden).Tags("auth")

	// ErrNotFound 资源不存在错误构建器
	ErrNotFound = Code(NotFound).Tags("resource")

	// ErrInternal 内部服务器错误构建器
	ErrInternal = Code(InternalServerError).Tags("server")

	// ErrConflict 资源冲突错误构建器
	ErrConflict = Code(Conflict).Tags("business")
)
