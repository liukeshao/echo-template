package errors

// 全局链式方法入口 - 为 AppErrorBuilder 的所有字段提供入口函数

// Code 设置错误码
func Code(code int) AppErrorBuilder {
	return newAppErrorBuilder().Code(code)
}

// In creates an error builder with a domain or feature category.
func In(doamin string) AppErrorBuilder {
	return newAppErrorBuilder().In(doamin)
}

// Tags 创建带标签的错误构建器
func Tags(tags ...string) AppErrorBuilder {
	return newAppErrorBuilder().Tags(tags...)
}

// With 创建带上下文的错误构建器
func With(key string, value any) AppErrorBuilder {
	return newAppErrorBuilder().With(key, value)
}

// 便捷的全局方法

// New 创建新的结构化错误
func New(message string) error {
	return newAppErrorBuilder().New(message)
}

// Errorf 创建格式化错误
func Errorf(format string, args ...any) error {
	return newAppErrorBuilder().Errorf(format, args...)
}

// Wrap 包装错误
func Wrap(err error) error {
	return newAppErrorBuilder().Wrap(err)
}

// Wrapf 包装错误并格式化消息
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return newAppErrorBuilder().Wrapf(err, format, args...)
}
