package handlers

import (
	"context"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/services"
)

var _ api.AuthHandler = (*Auth)(nil)

// Auth 认证处理器（纯 Go 业务编排层，实现 api.AuthHandler，零 net/http 依赖）。
type Auth struct {
	auth *services.AuthService
}

// NewAuth 构造 Auth 并注入依赖。
func NewAuth(i do.Injector) (*Auth, error) {
	return &Auth{
		auth: do.MustInvoke[*services.AuthService](i),
	}, nil
}

func (h *Auth) Register(ctx context.Context, req *api.RegisterRequest) (*api.TokenPair, error) {
	return h.auth.Register(ctx, req)
}

func (h *Auth) Login(ctx context.Context, req *api.LoginRequest) (*api.TokenPair, error) {
	return h.auth.Login(ctx, req)
}

func (h *Auth) RefreshToken(ctx context.Context, req *api.RefreshTokenRequest) (*api.TokenPair, error) {
	return h.auth.RefreshToken(ctx, req)
}

func (h *Auth) Logout(ctx context.Context) error {
	user := appctx.MustGetUserFromContext(ctx)
	return h.auth.Logout(ctx, user.ID)
}
