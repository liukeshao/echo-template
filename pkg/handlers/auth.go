package handlers

import (
	"context"
	"net/http"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/httpruntime"
	"github.com/liukeshao/echo-template/pkg/services"
)

var _ api.AuthHandler = (*AuthHandler)(nil)

// AuthHandler 认证处理器（实现 TypeSpec 生成的 api.AuthHandler 接口）。
type AuthHandler struct {
	auth *services.AuthService
}

// NewAuthHandler 构造 AuthHandler 并注入依赖。
func NewAuthHandler(i do.Injector) (*AuthHandler, error) {
	return &AuthHandler{
		auth: do.MustInvoke[*services.AuthService](i),
	}, nil
}

// Mount 将认证路由挂载到标准 *http.ServeMux（路由、鉴权、参数校验均由生成代码驱动）。
func (h *AuthHandler) Mount(mux *http.ServeMux, rt *httpruntime.Runtime, auth func(http.Handler) http.Handler) {
	api.MountAuthHandler(mux, h, rt, auth)
}

func (h *AuthHandler) Register(ctx context.Context, req *api.RegisterRequest) (*api.TokenPair, error) {
	return h.auth.Register(ctx, req)
}

func (h *AuthHandler) Login(ctx context.Context, req *api.LoginRequest) (*api.TokenPair, error) {
	return h.auth.Login(ctx, req)
}

func (h *AuthHandler) RefreshToken(ctx context.Context, req *api.RefreshTokenRequest) (*api.TokenPair, error) {
	return h.auth.RefreshToken(ctx, req)
}

func (h *AuthHandler) Logout(ctx context.Context) error {
	user := appctx.MustGetUserFromContext(ctx)
	return h.auth.Logout(ctx, user.ID)
}
