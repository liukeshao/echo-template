package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/samber/do/v2"
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/rest"
	"github.com/liukeshao/echo-template/pkg/services"
)

const defaultMaxBodyBytes = 2 << 20 // 2 MiB

// BuildRouter 构建 rest.Router，显式挂载全部业务端点与文档路由。
func BuildRouter(i do.Injector) (http.Handler, error) {
	cfg := do.MustInvoke[*config.Config](i)
	rt := do.MustInvoke[*rest.Runtime](i)
	authSvc := do.MustInvoke[*services.AuthService](i)

	opts := []rest.HandlerOption{
		rest.WithRuntime(rt),
		rest.WithAuthScheme("BearerAuth", BearerAuth(authSvc)),
	}

	router := rest.NewRouter(rt, rest.WithMaxBodyBytes(defaultMaxBodyBytes))
	router.Mount(
		api.NewAuthHandler(do.MustInvoke[*Auth](i), opts...),
		api.NewBooksHandler(do.MustInvoke[*Book](i), opts...),
		api.NewMeHandler(do.MustInvoke[*Me](i), opts...),
	)

	// 生产环境不暴露 API 文档与静态资源。
	if cfg.App.Environment != config.EnvProduction {
		router.Mount(do.MustInvoke[*Docs](i).Handler(rt))
	}

	return router, nil
}

// NewServer 构建路由并返回已接入 DI 生命周期（优雅停机）的 *rest.Server。
func NewServer(i do.Injector) (*rest.Server, error) {
	cfg := do.MustInvoke[*config.Config](i)
	handler, err := BuildRouter(i)
	if err != nil {
		return nil, err
	}
	return rest.NewServer(cfg, handler), nil
}

// BearerAuth 返回对应 TypeSpec @useAuth(BearerAuth) 的 rest.Authenticator：
// 验证 Bearer token 并将已认证 *ent.User 写入 context。
// token 不写入 context，防止被日志框架意外序列化后泄露。
func BearerAuth(authService *services.AuthService) rest.Authenticator {
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

		return appctx.WithUser(ctx, user), nil
	}
}

// extractTokenFromHeader 从 Authorization header 提取 Bearer token（scheme 大小写不敏感，遵循 RFC 9110）。
// header 缺失或格式非法时返回 AUTH_TOKEN_INVALID oops 错误（→ HTTP 401）。
func extractTokenFromHeader(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	scheme, rawToken, ok := strings.Cut(authHeader, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", oops.
			Code(codes.AuthTokenInvalid.String()).
			Public("missing or invalid Authorization header").
			Errorf("missing or invalid Authorization header")
	}

	token := strings.TrimSpace(rawToken)
	if token == "" {
		return "", oops.
			Code(codes.AuthTokenInvalid.String()).
			Public("bearer token is empty").
			Errorf("empty bearer token")
	}

	return token, nil
}

// mapSlice 将源切片逐元素映射为目标切片，保证返回值永不为 nil（空切片 JSON 序列化为 []）。
func mapSlice[In, Out any](in []In, fn func(In) Out) []Out {
	out := make([]Out, len(in))
	for i, v := range in {
		out[i] = fn(v)
	}
	return out
}
