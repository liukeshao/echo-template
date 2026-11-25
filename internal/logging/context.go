package logging

import "context"

type requestIDKey struct{}

// WithRequestID 在 context 中设置 request ID。
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// LookupRequestID 从 context 中获取 request ID 及是否存在标志。
func LookupRequestID(ctx context.Context) (string, bool) {
	requestID, ok := ctx.Value(requestIDKey{}).(string)
	return requestID, ok
}

// RequestID 从 context 中获取 request ID，如果不存在则返回空字符串。
func RequestID(ctx context.Context) string {
	requestID, _ := LookupRequestID(ctx)
	return requestID
}
