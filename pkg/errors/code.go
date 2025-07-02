package errors

// API状态码定义 - 整数错误码系统
const (
	// 成功
	OK = 0

	// 客户端错误 1xxxx
	BadRequest          = 10001 // 请求参数错误
	Unauthorized        = 10002 // 未授权
	Forbidden           = 10003 // 禁止访问
	NotFound            = 10004 // 资源不存在
	MethodNotAllowed    = 10005 // 方法不允许
	Conflict            = 10006 // 资源冲突
	UnprocessableEntity = 10007 // 数据验证失败
	TooManyRequests     = 10008 // 请求过于频繁

	// 服务器错误 2xxxx
	InternalServerError = 20001 // 内部服务器错误
	NotImplemented      = 20002 // 功能未实现
	BadGateway          = 20003 // 网关错误
	ServiceUnavailable  = 20004 // 服务不可用
	GatewayTimeout      = 20005 // 网关超时

	// 业务错误码 3xxxx
	DatabaseError      = 30001 // 数据库错误
	CacheError         = 30002 // 缓存错误
	ExternalAPIError   = 30003 // 外部API错误
	ValidationError    = 30004 // 业务验证错误
	BusinessLogicError = 30005 // 业务逻辑错误
)

// 预置的错误构建器
var (
	// ErrDatabase 数据库错误构建器
	ErrDatabase = Code(DatabaseError).Tags("database").Message("数据库操作失败")

	// ErrBadRequest 请求参数错误构建器
	ErrBadRequest = Code(BadRequest).Tags("client").Message("请求参数错误")

	// ErrUnauthorized 未授权错误构建器
	ErrUnauthorized = Code(Unauthorized).Tags("auth").Message("未授权访问")

	// ErrForbidden 禁止访问错误构建器
	ErrForbidden = Code(Forbidden).Tags("auth").Message("禁止访问")

	// ErrNotFound 资源不存在错误构建器
	ErrNotFound = Code(NotFound).Tags("resource").Message("资源不存在")

	// ErrInternal 内部服务器错误构建器
	ErrInternal = Code(InternalServerError).Tags("server").Message("内部服务器错误")

	// ErrConflict 资源冲突错误构建器
	ErrConflict = Code(Conflict).Tags("business").Message("资源冲突")

	// ErrValidation 数据验证错误构建器
	ErrValidation = Code(ValidationError).Tags("validation").Message("数据验证失败")

	// ErrBusinessLogic 业务逻辑错误构建器
	ErrBusinessLogic = Code(BusinessLogicError).Tags("business").Message("业务逻辑错误")

	// ErrCache 缓存错误构建器
	ErrCache = Code(CacheError).Tags("cache").Message("缓存操作失败")

	// ErrExternalAPI 外部API错误构建器
	ErrExternalAPI = Code(ExternalAPIError).Tags("external").Message("外部API调用失败")
)
