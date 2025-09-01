package appctx

import (
	"context"

	"github.com/liukeshao/echo-template/ent"
)

// contextKey 是用于 context 键的自定义类型，确保类型安全
type contextKey string

// 定义常量键，使用私有变量确保唯一性
const (
	requestIDKey contextKey = "request_id"
	userKey      contextKey = "user"
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

// WithUser 在 context 中设置用户信息
func WithUser(ctx context.Context, user *ent.User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// GetUserFromContext 从 context 中获取当前用户
func GetUserFromContext(ctx context.Context) (*ent.User, bool) {
	user, ok := ctx.Value(userKey).(*ent.User)
	return user, ok
}

// MustGetUserFromContext 从 context 中获取用户，如果不存在则panic（用于必须有用户的地方）
func MustGetUserFromContext(ctx context.Context) *ent.User {
	user, ok := GetUserFromContext(ctx)
	if !ok {
		panic("user not found in context")
	}
	return user
}
