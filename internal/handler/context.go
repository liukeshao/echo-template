package handler

import (
	"context"

	"github.com/liukeshao/echo-template/ent"
)

type userKey struct{}

// WithUser 在 context 中设置已认证用户信息。
func WithUser(ctx context.Context, user *ent.User) context.Context {
	return context.WithValue(ctx, userKey{}, user)
}

// User 从 context 中获取当前已认证用户。
func User(ctx context.Context) (*ent.User, bool) {
	user, ok := ctx.Value(userKey{}).(*ent.User)
	return user, ok && user != nil
}

// MustUser 从 context 中获取当前已认证用户，若不存在则 panic（用于受鉴权保护的处理器）。
func MustUser(ctx context.Context) *ent.User {
	user, ok := User(ctx)
	if !ok {
		panic("user not found in context")
	}
	return user
}
