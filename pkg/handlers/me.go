package handlers

import (
	"context"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/services"
)

var _ api.MeHandler = (*Me)(nil)

// Me 当前登录用户的处理器（纯 Go 业务编排层，实现 api.MeHandler，零 net/http 依赖）。
// PATCH /api/v1/me 由 TypeSpec @autoPatch(Me.get, Me.replace) 自动串联 GetMe + ReplaceMe。
type Me struct {
	me *services.MeService
}

// NewMe 构造 Me 并注入依赖。
func NewMe(i do.Injector) (*Me, error) {
	return &Me{
		me: do.MustInvoke[*services.MeService](i),
	}, nil
}

func (h *Me) GetMe(ctx context.Context) (*api.User, error) {
	user := appctx.MustGetUserFromContext(ctx)
	u := toUser(user)
	return &u, nil
}

func (h *Me) ReplaceMe(ctx context.Context, req *api.ReplaceMeRequest) (*api.User, error) {
	user := appctx.MustGetUserFromContext(ctx)
	updated, err := h.me.Replace(ctx, user.ID, req)
	if err != nil {
		return nil, err
	}
	u := toUser(updated)
	return &u, nil
}

func (h *Me) ChangePassword(ctx context.Context, req *api.ChangePasswordRequest) error {
	user := appctx.MustGetUserFromContext(ctx)
	return h.me.ChangePassword(ctx, user.ID, req)
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
