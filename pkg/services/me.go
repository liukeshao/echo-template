package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/samber/do/v2"
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/infra"
	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/liukeshao/echo-template/pkg/utils"
)

// MeService 用户服务
type MeService struct {
	orm *ent.Client
}

// NewMeService 构造 MeService 并从 DI 容器注入依赖。
func NewMeService(i do.Injector) (*MeService, error) {
	return &MeService{
		orm: do.MustInvoke[*infra.ORM](i).Client,
	}, nil
}

// meInternal 构造携带 "me" 标签的 ME_INTERNAL_ERROR oops 错误。
func meInternal(err error, op string) error {
	return oops.
		Code(codes.MeInternal.String()).
		Tags("me").
		Wrapf(err, "%s", op)
}

// UpdateUsername 检查用户名唯一性后更新已登录用户的用户名。
func (s *MeService) UpdateUsername(ctx context.Context, userID string, req *types.UpdateUsernameRequest) error {
	exists, err := s.orm.User.Query().
		Where(
			user.UsernameEQ(req.Username),
			user.IDNEQ(userID),
		).
		Exist(ctx)
	if err != nil {
		return meInternal(err, "checking username uniqueness")
	}
	if exists {
		return oops.
			Code(codes.MeUsernameTaken.String()).
			With(codes.CtxKeyMetadata, map[string]string{"username": req.Username}).
			Public(fmt.Sprintf("the username %q is already taken", req.Username)).
			Errorf("username taken: %s", req.Username)
	}

	if err := s.orm.User.UpdateOneID(userID).SetUsername(req.Username).Exec(ctx); err != nil {
		return meInternal(err, "updating username")
	}
	return nil
}

// UpdateEmail 检查邮箱唯一性后更新已登录用户的邮箱。
func (s *MeService) UpdateEmail(ctx context.Context, userID string, req *types.UpdateEmailRequest) error {
	exists, err := s.orm.User.Query().
		Where(
			user.EmailEQ(req.Email),
			user.IDNEQ(userID),
		).
		Exist(ctx)
	if err != nil {
		return meInternal(err, "checking email uniqueness")
	}
	if exists {
		return oops.
			Code(codes.MeEmailTaken.String()).
			With(codes.CtxKeyMetadata, map[string]string{"email": req.Email}).
			Public(fmt.Sprintf("the email %q is already in use", req.Email)).
			Errorf("email taken: %s", req.Email)
	}

	if err := s.orm.User.UpdateOneID(userID).SetEmail(req.Email).Exec(ctx); err != nil {
		return meInternal(err, "updating email")
	}
	return nil
}

// ChangePassword 修改已登录用户的密码。
func (s *MeService) ChangePassword(ctx context.Context, userID string, req *types.ChangePasswordRequest) error {
	u, err := s.orm.User.Query().
		Where(user.IDEQ(userID)).
		First(ctx)
	if err != nil {
		return meInternal(err, "fetching user for password change")
	}

	if err := utils.VerifyPassword(u.PasswordHash, req.OldPassword); err != nil {
		slog.WarnContext(ctx, "invalid old password", "user_id", userID)
		return oops.
			Code(codes.MeInvalidPassword.String()).
			Public("current password is incorrect").
			Errorf("invalid old password: user_id=%s", userID)
	}

	newHash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		return meInternal(err, "hashing new password")
	}

	if err := s.orm.User.UpdateOneID(userID).SetPasswordHash(newHash).Exec(ctx); err != nil {
		return meInternal(err, "updating password")
	}

	return nil
}
