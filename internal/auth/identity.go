// Package auth 提供认证基础设施：不可变请求主体（Principal）、无状态 Access JWT 签发/验签与 Bearer 认证中间件适配。
package auth

import "context"

// Principal 表示已通过认证的请求主体（值类型，不含 ORM 实体或密码哈希等敏感字段）。
type Principal struct {
	UserID  string
	TokenID string
}

type principalKey struct{}

// WithPrincipal 将已认证主体写入 context。
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFromContext 从 context 中读取当前已认证主体。
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok && p.UserID != ""
}

// MustPrincipal 从 context 中读取当前已认证主体；若缺失则 panic（仅用于受鉴权保护的处理器）。
func MustPrincipal(ctx context.Context) Principal {
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		panic("auth principal not found in context")
	}
	return p
}
