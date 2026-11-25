package handler

import (
	"context"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/internal/service"
	"github.com/liukeshao/echo-template/pkg/api"
)

var _ api.AuthHandler = (*Auth)(nil)

// Auth 认证处理器（纯 Go 业务编排层，实现 api.AuthHandler，零 net/http 依赖）。
type Auth struct {
	auth *service.Auth
}

// NewAuth 构造 Auth 并注入依赖。
func NewAuth(i do.Injector) (*Auth, error) {
	return &Auth{
		auth: do.MustInvoke[*service.Auth](i),
	}, nil
}

func (h *Auth) Register(ctx context.Context, req *api.RegisterRequest) (*api.TokenPair, error) {
	tp, err := h.auth.Register(ctx, req)
	if err != nil {
		return nil, err
	}
	res := toTokenPair(tp)
	return &res, nil
}

func (h *Auth) Login(ctx context.Context, req *api.LoginRequest) (*api.TokenPair, error) {
	tp, err := h.auth.Login(ctx, req)
	if err != nil {
		return nil, err
	}
	res := toTokenPair(tp)
	return &res, nil
}

func (h *Auth) RefreshToken(ctx context.Context, req *api.RefreshTokenRequest) (*api.TokenPair, error) {
	tp, err := h.auth.RefreshToken(ctx, req)
	if err != nil {
		return nil, err
	}
	res := toTokenPair(tp)
	return &res, nil
}

func (h *Auth) Logout(ctx context.Context) error {
	user := MustUser(ctx)
	return h.auth.Logout(ctx, user.ID)
}

func toTokenPair(tp *service.TokenPair) api.TokenPair {
	return api.TokenPair{
		AccessToken:  tp.AccessToken,
		RefreshToken: tp.RefreshToken,
		ExpiresAt:    tp.ExpiresAt,
	}
}
