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
	"github.com/liukeshao/echo-template/pkg/httperr"
	"github.com/liukeshao/echo-template/pkg/httpkit"
	"github.com/liukeshao/echo-template/static"
)

const defaultMaxBodyBytes = 2 << 20 // 2 MiB

// BuildRouter 构建 httpkit.Router，显式编排全局中间件链并挂载全部业务端点与文档路由。
func BuildRouter(i do.Injector) (http.Handler, error) {
	cfg := do.MustInvoke[*config.Config](i)
	tr := do.MustInvoke[i18n.Translator](i)
	authSvc := do.MustInvoke[*service.Auth](i)

	encodeErr := NewProblemEncoder(tr)

	opts := []httpkit.HandlerOption{
		httpkit.WithErrorEncoder(encodeErr),
		httpkit.WithAuthScheme("BearerAuth", BearerAuth(authSvc)),
	}

	router := httpkit.NewRouter()
	router.SetErrorEncoder(encodeErr)
	router.Use(
		httpkit.RequestID(),
		Locale(tr),
		httpkit.AccessLog(),
		httpkit.Recover(encodeErr),
		httpkit.BodyLimit(defaultMaxBodyBytes),
	)
	router.Mount(
		api.NewAuthHandler(do.MustInvoke[*handler.Auth](i), opts...),
		api.NewBooksHandler(do.MustInvoke[*handler.Book](i), opts...),
		api.NewMeHandler(do.MustInvoke[*handler.Me](i), opts...),
	)

	// 生产环境不暴露 API 文档与静态资源。
	if cfg.App.Environment != config.EnvProduction {
		router.FileFS("/docs", "index.html", static.DocsFS)
		router.FileFS("/openapi.yaml", "openapi/openapi.yaml", static.DocsFS)
		router.StaticFS("/static", static.AssetsFS)
	}

	return router, nil
}

// Locale 协商请求语言（?lang= > Accept-Language > Default）并写入 context。
func Locale(tr i18n.Translator) httpkit.Middleware {
	if tr == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tag := tr.Match(
				r.Header.Get("Accept-Language"),
				r.URL.Query().Get("lang"),
			)
			next.ServeHTTP(w, r.WithContext(i18n.WithLocale(r.Context(), tag)))
		})
	}
}

// BearerAuth 返回对应 TypeSpec @useAuth(BearerAuth) 的 httpkit.Authenticator：
// 验证 Bearer token 并将已认证 *ent.User 写入 context。
// token 不写入 context，防止被日志框架意外序列化后泄露。
func BearerAuth(authService *service.Auth) httpkit.Authenticator {
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
// header 缺失或格式非法时返回 401 Unauthorized oops 错误（附带 PreconditionInfo）。
func extractTokenFromHeader(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if strings.TrimSpace(authHeader) == "" {
		return "", bearerHeaderError(httperr.PreconditionMissingHeader)
	}

	scheme, rawToken, ok := strings.Cut(authHeader, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", bearerHeaderError(httperr.PreconditionMalformedHeader)
	}

	token := strings.TrimSpace(rawToken)
	if token == "" {
		return "", bearerHeaderError(httperr.PreconditionEmptyToken)
	}

	return token, nil
}

func bearerHeaderError(reason string) error {
	return oops.
		In("auth").
		With(httperr.MetaKey, httperr.PreconditionInfo{
			Type:    reason,
			Subject: "authorization",
			Detail:  "Bearer",
		}).
		Wrap(httperr.Unauthorized)
}
