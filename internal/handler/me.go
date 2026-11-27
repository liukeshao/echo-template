package handler

import (
	"context"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/internal/auth"
	"github.com/liukeshao/echo-template/internal/service"
	"github.com/liukeshao/echo-template/pkg/api"
)

var _ api.MeHandler = (*Me)(nil)

// Me 当前登录用户的单例资源处理器（纯 Go 业务编排层，实现 api.MeHandler，零 net/http 依赖）。
// 编排 service.User（资料读写）与 service.Auth（密码凭证变更），
// PATCH /api/v1/me 由 TypeSpec @autoPatch(Me.getMe, Me.replaceMe) 自动串联 GetMe + ReplaceMe。
type Me struct {
	user *service.User
	auth *service.Auth
}

// NewMe 构造 Me 并注入依赖。
func NewMe(i do.Injector) (*Me, error) {
	return &Me{
		user: do.MustInvoke[*service.User](i),
		auth: do.MustInvoke[*service.Auth](i),
	}, nil
}

func (h *Me) GetMe(ctx context.Context) (*api.User, error) {
	principal := auth.MustPrincipal(ctx)
	u, err := h.user.Get(ctx, principal.UserID)
	if err != nil {
		return nil, err
	}
	res := toUser(u)
	return &res, nil
}

func (h *Me) ReplaceMe(ctx context.Context, req *api.ReplaceMeRequest) (*api.User, error) {
	principal := auth.MustPrincipal(ctx)
	updated, err := h.user.Replace(ctx, principal.UserID, req)
	if err != nil {
		return nil, err
	}
	res := toUser(updated)
	return &res, nil
}

func (h *Me) ChangePassword(ctx context.Context, req *api.ChangePasswordRequest) error {
	principal := auth.MustPrincipal(ctx)
	return h.auth.ChangePassword(ctx, principal.UserID, req)
}

// toUser 将 *ent.User 映射为对外的资源视图 api.User（不含密码、删除标记等）。
func toUser(u *ent.User) api.User {
	return api.User{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		Status:      api.UserStatus(u.Status),
		LastLoginAt: u.LastLoginAt,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}
