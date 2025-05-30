package context

import "context"

// contextKey 是用于 context 键的自定义类型，确保类型安全
type contextKey string

// 定义常量键，使用私有变量确保唯一性
const (
	requestIDKey contextKey = "request_id"
)

// WithRequestID 在 context 中设置 request ID
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestIDFromContext 从 context 中获取 request ID
func GetRequestIDFromContext(ctx context.Context) (string, bool) {
	requestID, ok := ctx.Value(requestIDKey).(string)
	return requestID, ok
}

// MustGetRequestIDFromContext 从 context 中获取 request ID，如果不存在则返回空字符串
func MustGetRequestIDFromContext(ctx context.Context) string {
	requestID, _ := GetRequestIDFromContext(ctx)
	return requestID
}
