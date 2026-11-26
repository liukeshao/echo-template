package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/samber/do/v2"
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/internal/handler"
	"github.com/liukeshao/echo-template/internal/i18n"
	"github.com/liukeshao/echo-template/internal/service"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/rest"
)

const defaultMaxBodyBytes = 2 << 20 // 2 MiB

// BuildRouter 构建 rest.Router，显式编排全局中间件链并挂载全部业务端点与文档路由。
func BuildRouter(i do.Injector) (http.Handler, error) {
	cfg := do.MustInvoke[*config.Config](i)
	tr := do.MustInvoke[i18n.Translator](i)
	authSvc := do.MustInvoke[*service.Auth](i)

	encodeErr := NewProblemEncoder(tr)

	opts := []rest.HandlerOption{
		rest.WithErrorEncoder(encodeErr),
		rest.WithAuthScheme("BearerAuth", BearerAuth(authSvc)),
	}

	router := rest.NewRouter(rest.WithRouterErrorEncoder(encodeErr))
	router.Use(
		RequestID(),
		Locale(tr),
		rest.AccessLog(),
		rest.Recover(encodeErr),
		rest.BodyLimit(defaultMaxBodyBytes),
	)
	router.Mount(
		api.NewAuthHandler(do.MustInvoke[*handler.Auth](i), opts...),
		api.NewBooksHandler(do.MustInvoke[*handler.Book](i), opts...),
		api.NewMeHandler(do.MustInvoke[*handler.Me](i), opts...),
	)

	// 生产环境不暴露 API 文档与静态资源。
	if cfg.App.Environment != config.EnvProduction {
		router.Mount(do.MustInvoke[*Docs](i).Handler(encodeErr))
	}

	return router, nil
}

// BearerAuth 返回对应 TypeSpec @useAuth(BearerAuth) 的 rest.Authenticator：
// 验证 Bearer token 并将已认证 *ent.User 写入 context。
// token 不写入 context，防止被日志框架意外序列化后泄露。
func BearerAuth(authService *service.Auth) rest.Authenticator {
	return func(ctx context.Context, r *http.Request) (context.Context, error) {
		tokenString, err := extractTokenFromHeader(r)
		if err != nil {
			slog.WarnContext(ctx, "auth failed: extract token", "error", err)
			return ctx, err
		}

		user, err := authService.AuthenticateUser(ctx, tokenString)
		if err != nil {
			return ctx, err
		}

		return handler.WithUser(ctx, user), nil
	}
}

// extractTokenFromHeader 从 Authorization header 提取 Bearer token（scheme 大小写不敏感，遵循 RFC 9110）。
// header 缺失或格式非法时返回 UNAUTHORIZED oops 错误（→ HTTP 401 + PreconditionInfo）。
func extractTokenFromHeader(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if strings.TrimSpace(authHeader) == "" {
		return "", oops.
			In("auth").
			Code(rest.CodeUnauthorized).
			With(rest.MetaKey, rest.PreconditionInfo{
				Type:    rest.PreconditionMissingHeader,
				Subject: "authorization",
				Detail:  "Bearer",
			}).
			Wrap(rest.CodeUnauthorized)
	}

	scheme, rawToken, ok := strings.Cut(authHeader, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", oops.
			In("auth").
			Code(rest.CodeUnauthorized).
			With(rest.MetaKey, rest.PreconditionInfo{
				Type:    rest.PreconditionMalformedHeader,
				Subject: "authorization",
				Detail:  "Bearer",
			}).
			Wrap(rest.CodeUnauthorized)
	}

	token := strings.TrimSpace(rawToken)
	if token == "" {
		return "", oops.
			In("auth").
			Code(rest.CodeUnauthorized).
			With(rest.MetaKey, rest.PreconditionInfo{
				Type:    rest.PreconditionEmptyToken,
				Subject: "authorization",
				Detail:  "Bearer",
			}).
			Wrap(rest.CodeUnauthorized)
	}

	return token, nil
}
