package appctx

import (
	"context"

	"golang.org/x/text/language"

	"github.com/liukeshao/echo-template/ent"
)

// contextKey 是用于 context 键的自定义类型，确保类型安全
type contextKey string

// 定义常量键，使用私有变量确保唯一性
const (
	requestIDKey contextKey = "request_id"
	userKey      contextKey = "user"
	localeKey    contextKey = "locale"
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

// RequestIDFromContext 从 context 中获取 request ID，如果不存在则返回空字符串。
func RequestIDFromContext(ctx context.Context) string {
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
	return user, ok && user != nil
}

// MustGetUserFromContext 从 context 中获取用户，如果不存在则panic（用于必须有用户的地方）
func MustGetUserFromContext(ctx context.Context) *ent.User {
	user, ok := GetUserFromContext(ctx)
	if !ok {
		panic("user not found in context")
	}
	return user
}

// WithLocale 在 context 中设置已协商好的 BCP-47 语言标签。
// 由 rest.Runtime.Locale 中间件在请求入口写入，供 rest.Runtime.WriteProblem 等下游消费。
func WithLocale(ctx context.Context, tag language.Tag) context.Context {
	return context.WithValue(ctx, localeKey, tag)
}

// LocaleFromContext 从 context 中读取语言标签；未设置时返回 fallback。
// 采用 fallback 参数而非 ok 布尔，简化调用点。
func LocaleFromContext(ctx context.Context, fallback language.Tag) language.Tag {
	if tag, ok := ctx.Value(localeKey).(language.Tag); ok {
		return tag
	}
	return fallback
}
