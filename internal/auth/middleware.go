package auth

import (
	"context"

	"github.com/liukeshao/go-rest-template/pkg/httpkit"
)

// BearerAuthenticator 返回无状态 JWT 认证器，零 DB 查询并将 Principal 写入 context。
func BearerAuthenticator(j *JWT) httpkit.Authenticator {
	return httpkit.BearerAuth(func(ctx context.Context, token string) (context.Context, error) {
		principal, err := j.ParseAccessToken(token)
		if err != nil {
			return ctx, err
		}
		return WithPrincipal(ctx, principal), nil
	})
}
