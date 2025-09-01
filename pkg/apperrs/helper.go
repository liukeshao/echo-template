package apperrs

import (
	"strconv"

	"github.com/samber/oops"
)

// 全局配置
func init() {
	// 设置堆栈跟踪深度
	oops.StackTraceMaxDepth = 10
	// 显示源代码片段（开发环境）
	oops.SourceFragmentsHidden = false
}

// 业务错误码定义 - 基于项目规范
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

// 预定义的错误构建器 - 遵循 oops 最佳实践
var (
	// ErrDatabase 数据库错误构建器
	ErrDatabase = oops.
			Code(strconv.Itoa(DatabaseError)).
			In("database").
			Tags("database").
			Hint("检查数据库连接和查询语法")

	// ErrBadRequest 请求参数错误构建器
	ErrBadRequest = oops.
			Code(strconv.Itoa(BadRequest)).
			In("validation").
			Tags("client", "request").
			Public("请求参数错误")

	// ErrUnauthorized 未授权错误构建器
	ErrUnauthorized = oops.
			Code(strconv.Itoa(Unauthorized)).
			In("auth").
			Tags("auth", "security").
			Public("未授权访问")

	// ErrForbidden 禁止访问错误构建器
	ErrForbidden = oops.
			Code(strconv.Itoa(Forbidden)).
			In("auth").
			Tags("auth", "security").
			Public("禁止访问")

	// ErrNotFound 资源不存在错误构建器
	ErrNotFound = oops.
			Code(strconv.Itoa(NotFound)).
			In("resource").
			Tags("resource").
			Public("资源不存在")

	// ErrInternal 内部服务器错误构建器
	ErrInternal = oops.
			Code(strconv.Itoa(InternalServerError)).
			In("server").
			Tags("server", "internal").
			Public("内部服务器错误")

	// ErrConflict 资源冲突错误构建器
	ErrConflict = oops.
			Code(strconv.Itoa(Conflict)).
			In("business").
			Tags("business", "conflict").
			Public("资源冲突")

	// ErrValidation 数据验证错误构建器
	ErrValidation = oops.
			Code(strconv.Itoa(ValidationError)).
			In("validation").
			Tags("validation", "business").
			Public("数据验证失败")

	// ErrBusinessLogic 业务逻辑错误构建器
	ErrBusinessLogic = oops.
				Code(strconv.Itoa(BusinessLogicError)).
				In("business").
				Tags("business", "logic").
				Public("业务逻辑错误")

	// ErrCache 缓存错误构建器
	ErrCache = oops.
			Code(strconv.Itoa(CacheError)).
			In("cache").
			Tags("cache").
			Hint("检查缓存服务状态")

	// ErrExternalAPI 外部API错误构建器
	ErrExternalAPI = oops.
			Code(strconv.Itoa(ExternalAPIError)).
			In("external").
			Tags("external", "api").
			Hint("检查外部服务状态和网络连接")
)
