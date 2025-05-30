package context

import "context"

// requestIDContextKey is the key used to store request ID in context
type requestIDContextKey struct{}

// RequestIDKey is the key for accessing request ID from context
var RequestIDKey = &requestIDContextKey{}

// GetRequestIDFromContext 从标准包的 context 中获取 request ID
func GetRequestIDFromContext(ctx context.Context) (string, bool) {
	requestID, ok := ctx.Value(RequestIDKey).(string)
	return requestID, ok
}

// MustGetRequestIDFromContext 从标准包的 context 中获取 request ID，如果不存在则返回空字符串
func MustGetRequestIDFromContext(ctx context.Context) string {
	requestID, _ := GetRequestIDFromContext(ctx)
	return requestID
}
