package auth

import (
	"context"

	"github.com/liukeshao/echo-template/pkg/httpkit"
)

// BearerAuthenticator 返回对应 TypeSpec @useAuth(BearerAuth) 的无状态 JWT 认证器：
// 提取 Authorization Bearer 令牌并在内存中验签（零数据库查询），校验通过后将 Principal 写入 context。
func BearerAuthenticator(j *JWT) httpkit.Authenticator {
	return httpkit.BearerAuth(func(ctx context.Context, token string) (context.Context, error) {
		principal, err := j.ParseAccessToken(token)
		if err != nil {
			return ctx, err
		}
		return WithPrincipal(ctx, principal), nil
	})
}
