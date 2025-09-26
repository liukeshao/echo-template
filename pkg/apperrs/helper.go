package apperrs

import (
	"github.com/samber/oops"
)

// 全局配置
func init() {
	// 设置堆栈跟踪深度
	oops.StackTraceMaxDepth = 10
	// 显示源代码片段（开发环境）
	oops.SourceFragmentsHidden = false
}

// 预定义的错误构建器 - 遵循 oops 最佳实践
var (
	// ErrDatabase 数据库错误构建器
	ErrDatabase = oops.
			Code(CodeDatabaseError.ToString()).
			In("database").
			Tags(TagDatabase).
			Hint("检查数据库连接和查询语法")

	// ErrBadRequest 请求参数错误构建器
	ErrBadRequest = oops.
			Code(CodeBadRequest.ToString()).
			In("validation").
			Tags(TagClient, TagRequest).
			Public("请求参数错误")

	// ErrUnauthorized 未授权错误构建器
	ErrUnauthorized = oops.
			Code(CodeUnauthorized.ToString()).
			In("auth").
			Tags(TagAuth, TagSecurity).
			Public("未授权访问")

	// ErrForbidden 禁止访问错误构建器
	ErrForbidden = oops.
			Code(CodeForbidden.ToString()).
			In("auth").
			Tags(TagAuth, TagSecurity).
			Public("禁止访问")

	// ErrNotFound 资源不存在错误构建器
	ErrNotFound = oops.
			Code(CodeNotFound.ToString()).
			In("resource").
			Tags(TagResource).
			Public("资源不存在")

	// ErrInternal 内部服务器错误构建器
	ErrInternal = oops.
			Code(CodeInternalServerError.ToString()).
			In("server").
			Tags(TagServer, TagInternal).
			Public("内部服务器错误")

	// ErrConflict 资源冲突错误构建器
	ErrConflict = oops.
			Code(CodeConflict.ToString()).
			In("business").
			Tags(TagBusiness, TagConflict).
			Public("资源冲突")

	// ErrValidation 数据验证错误构建器
	ErrValidation = oops.
			Code(CodeValidationError.ToString()).
			In("validation").
			Tags(TagValidation, TagBusiness).
			Public("数据验证失败")

	// ErrBusinessLogic 业务逻辑错误构建器
	ErrBusinessLogic = oops.
				Code(CodeBusinessLogicError.ToString()).
				In("business").
				Tags(TagBusiness, TagLogic).
				Public("业务逻辑错误")

	// ErrCache 缓存错误构建器
	ErrCache = oops.
			Code(CodeCacheError.ToString()).
			In("cache").
			Tags(TagCache).
			Hint("检查缓存服务状态")

	// ErrExternalAPI 外部API错误构建器
	ErrExternalAPI = oops.
			Code(CodeExternalAPIError.ToString()).
			In("external").
			Tags(TagExternal, TagAPI).
			Hint("检查外部服务状态和网络连接")
)
