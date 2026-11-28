package handler

import (
	"context"

	"github.com/samber/do/v2"

	"github.com/liukeshao/go-rest-template/ent"
	"github.com/liukeshao/go-rest-template/internal/auth"
	"github.com/liukeshao/go-rest-template/internal/service"
	"github.com/liukeshao/go-rest-template/pkg/api"
)

var _ api.MeHandler = (*Me)(nil)

type Me struct {
	user *service.User
	auth *service.Auth
}

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
	updated, err := h.user.Replace(ctx, principal.UserID, service.ReplaceUserInput{
		Username: req.Username,
		Email:    req.Email,
	})
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
