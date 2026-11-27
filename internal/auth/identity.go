package auth

import "context"

type Principal struct {
	UserID  string
	TokenID string
}

type principalKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok && p.UserID != ""
}

func MustPrincipal(ctx context.Context) Principal {
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		panic("auth principal not found in context")
	}
	return p
}
