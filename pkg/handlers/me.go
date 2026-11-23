package handlers

import (
	"context"
	"net/http"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/appctx"
	"github.com/liukeshao/echo-template/pkg/httpruntime"
	"github.com/liukeshao/echo-template/pkg/services"
)

var _ api.MeHandler = (*MeHandler)(nil)

// MeHandler 当前登录用户的处理器（实现 TypeSpec 生成的 api.MeHandler 接口）。
// PATCH /api/v1/me 由 TypeSpec @autoPatch(Me.get, Me.replace) 自动串联 GetMe + ReplaceMe。
type MeHandler struct {
	me *services.MeService
}

// NewMeHandler 构造 MeHandler 并注入依赖。
func NewMeHandler(i do.Injector) (*MeHandler, error) {
	return &MeHandler{
		me: do.MustInvoke[*services.MeService](i),
	}, nil
}

// Mount 将 /api/v1/me 路由挂载到标准 *http.ServeMux。
func (h *MeHandler) Mount(mux *http.ServeMux, rt *httpruntime.Runtime, auth func(http.Handler) http.Handler) {
	api.MountMeHandler(mux, h, rt, auth)
}

func (h *MeHandler) GetMe(ctx context.Context) (*api.User, error) {
	user := appctx.MustGetUserFromContext(ctx)
	u := toUser(user)
	return &u, nil
}

func (h *MeHandler) ReplaceMe(ctx context.Context, req *api.ReplaceMeRequest) (*api.User, error) {
	user := appctx.MustGetUserFromContext(ctx)
	updated, err := h.me.Replace(ctx, user.ID, req)
	if err != nil {
		return nil, err
	}
	u := toUser(updated)
	return &u, nil
}

func (h *MeHandler) ChangePassword(ctx context.Context, req *api.ChangePasswordRequest) error {
	user := appctx.MustGetUserFromContext(ctx)
	return h.me.ChangePassword(ctx, user.ID, req)
}

// toUser 将 *ent.User 映射为对外的资源视图 api.User（不含密码、删除标记等）。
func toUser(u *ent.User) api.User {
	return api.User{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		Status:      string(u.Status),
		LastLoginAt: u.LastLoginAt,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}
