package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/samber/do/v2"
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/token"
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

// Replace 全量替换当前用户的可写资料（username / email），Kubernetes 风格的幂等 PUT。
//
// 唯一性约束：username 与 email 全局唯一（排除当前用户自身），冲突时返回
// ME_USERNAME_TAKEN / ME_EMAIL_TAKEN，对应 HTTP 409。
//
// PATCH handler 通过"GET → apply patch → Replace"流程复用本方法，
// 故 RFC 6902 / 7386 / Shorthand 三种 patch 协议共享同一份服务端校验逻辑。
func (s *MeService) Replace(ctx context.Context, userID string, req *types.ReplaceMeRequest) (*ent.User, error) {
	usernameTaken, err := s.orm.User.Query().
		Where(user.UsernameEQ(req.Username), user.IDNEQ(userID)).
		Exist(ctx)
	if err != nil {
		return nil, meInternal(err, "checking username uniqueness")
	}
	if usernameTaken {
		return nil, oops.
			Code(codes.MeUsernameTaken.String()).
			With(codes.CtxKeyMetadata, map[string]string{"username": req.Username}).
			Public(fmt.Sprintf("the username %q is already taken", req.Username)).
			Errorf("username taken: %s", req.Username)
	}

	emailTaken, err := s.orm.User.Query().
		Where(user.EmailEQ(req.Email), user.IDNEQ(userID)).
		Exist(ctx)
	if err != nil {
		return nil, meInternal(err, "checking email uniqueness")
	}
	if emailTaken {
		return nil, oops.
			Code(codes.MeEmailTaken.String()).
			With(codes.CtxKeyMetadata, map[string]string{"email": req.Email}).
			Public(fmt.Sprintf("the email %q is already in use", req.Email)).
			Errorf("email taken: %s", req.Email)
	}

	u, err := s.orm.User.UpdateOneID(userID).
		SetUsername(req.Username).
		SetEmail(req.Email).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			if taken, checkErr := s.orm.User.Query().Where(user.UsernameEQ(req.Username), user.IDNEQ(userID)).Exist(ctx); checkErr == nil && taken {
				return nil, oops.
					Code(codes.MeUsernameTaken.String()).
					With(codes.CtxKeyMetadata, map[string]string{"username": req.Username}).
					Public(fmt.Sprintf("the username %q is already taken", req.Username)).
					Errorf("username taken: %s", req.Username)
			}
			if taken, checkErr := s.orm.User.Query().Where(user.EmailEQ(req.Email), user.IDNEQ(userID)).Exist(ctx); checkErr == nil && taken {
				return nil, oops.
					Code(codes.MeEmailTaken.String()).
					With(codes.CtxKeyMetadata, map[string]string{"email": req.Email}).
					Public(fmt.Sprintf("the email %q is already in use", req.Email)).
					Errorf("email taken: %s", req.Email)
			}
		}
		return nil, meInternal(err, "replacing user profile")
	}
	return u, nil
}

// ChangePassword 修改已登录用户的密码（自定义子资源动作）。
//
// 不通过 PUT /me 暴露：密码不出现在 UserInfo 响应中，
// 且需要校验 old_password 才能轮换，这与"幂等全量替换"语义不兼容。
// 修改成功后在同一事务内撤销该用户所有活跃令牌，迫使所有会话使用新密码重新认证。
func (s *MeService) ChangePassword(ctx context.Context, userID string, req *types.ChangePasswordRequest) (err error) {
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

	tx, err := s.orm.Tx(ctx)
	if err != nil {
		return meInternal(err, "beginning password change transaction")
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = tx.User.UpdateOneID(userID).SetPasswordHash(newHash).Exec(ctx); err != nil {
		return meInternal(err, "updating password")
	}

	if _, err = tx.Token.Update().
		Where(token.UserID(userID), token.IsRevoked(false)).
		SetIsRevoked(true).
		Save(ctx); err != nil {
		return meInternal(err, "revoking tokens after password change")
	}

	if err = tx.Commit(); err != nil {
		return meInternal(err, "committing password change transaction")
	}

	return nil
}
