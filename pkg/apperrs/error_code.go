package apperrs

import (
	"fmt"
	"strconv"
)

// Code 错误码类型，提供类型安全和字符串转换功能
type Code int

// ToString 返回错误码的字符串表示
func (c Code) ToString() string {
	return strconv.Itoa(int(c))
}

// ToInt 返回错误码的整数值
func (c Code) ToInt() int {
	return int(c)
}

// String 实现fmt.Stringer接口，内部调用ToString
func (c Code) String() string {
	return c.ToString()
}

// FromString 从字符串解析错误码，供oops转response使用
func FromString(s string) (Code, error) {
	if s == "" {
		return CodeInternalServerError, fmt.Errorf("空错误码字符串")
	}

	code, err := strconv.Atoi(s)
	if err != nil {
		return CodeInternalServerError, fmt.Errorf("无效的错误码格式: %s", s)
	}

	return Code(code), nil
}

// FromInt 从整数创建错误码
func FromInt(i int) Code {
	return Code(i)
}

// MustFromString 从字符串解析错误码，解析失败时返回默认的内部服务器错误码
func MustFromString(s string) Code {
	code, err := FromString(s)
	if err != nil {
		return CodeInternalServerError
	}
	return code
}

// 基本状态码
const CodeOK Code = 0 // 成功

// 业务错误码定义 - 基于项目规范
const CodeUnknownError Code = -1 // 未知错误

// 客户端错误 1xxxx
const (
	CodeBadRequest          Code = 10001 // 请求参数错误
	CodeUnauthorized        Code = 10002 // 未授权
	CodeForbidden           Code = 10003 // 禁止访问
	CodeNotFound            Code = 10004 // 资源不存在
	CodeMethodNotAllowed    Code = 10005 // 方法不允许
	CodeConflict            Code = 10006 // 资源冲突
	CodeUnprocessableEntity Code = 10007 // 数据验证失败
	CodeTooManyRequests     Code = 10008 // 请求过于频繁
)

// 服务器错误 2xxxx
const (
	CodeInternalServerError Code = 20001 // 内部服务器错误
	CodeNotImplemented      Code = 20002 // 功能未实现
	CodeBadGateway          Code = 20003 // 网关错误
	CodeServiceUnavailable  Code = 20004 // 服务不可用
	CodeGatewayTimeout      Code = 20005 // 网关超时
)

// 业务错误码 3xxxx
const (
	CodeDatabaseError      Code = 30001 // 数据库错误
	CodeCacheError         Code = 30002 // 缓存错误
	CodeExternalAPIError   Code = 30003 // 外部API错误
	CodeValidationError    Code = 30004 // 业务验证错误
	CodeBusinessLogicError Code = 30005 // 业务逻辑错误
)
