package service

import (
	"context"
	"log/slog"

	"github.com/samber/do/v2"
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/token"
	"github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/internal/codes"
	"github.com/liukeshao/echo-template/internal/database"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/rest"
)

var errMe = oops.In("me")

// Me 用户服务
type Me struct {
	orm        *ent.Client
	bcryptCost int
}

// NewMe 构造 Me 并从 DI 容器注入依赖。
func NewMe(i do.Injector) (*Me, error) {
	cfg := do.MustInvoke[*config.Config](i)
	return &Me{
		orm:        do.MustInvoke[*database.ORM](i).Client,
		bcryptCost: resolveBcryptCost(cfg.Auth.BcryptCost),
	}, nil
}

func meInternal(err error, op string) error {
	return errMe.
		Tags("me").
		Code(codes.MeInternalError).
		Wrapf(err, "%s", op)
}

func meUsernameTaken(userID, username string) error {
	return errMe.
		Code(codes.MeUsernameTaken).
		User(userID).
		With(rest.MetaKey, rest.ResourceInfo{
			ResourceType: "user",
			ResourceName: username,
			Field:        "username",
		}).
		New("username taken")
}

func meEmailTaken(userID, email string) error {
	return errMe.
		Code(codes.MeEmailTaken).
		User(userID).
		With(rest.MetaKey, rest.ResourceInfo{
			ResourceType: "user",
			ResourceName: email,
			Field:        "email",
		}).
		New("email taken")
}

// Replace 全量替换当前用户的可写资料（username / email），Kubernetes 风格的幂等 PUT。
func (s *Me) Replace(ctx context.Context, userID string, req *api.ReplaceMeRequest) (*ent.User, error) {
	usernameTaken, err := s.orm.User.Query().
		Where(user.UsernameEQ(req.Username), user.IDNEQ(userID)).
		Exist(ctx)
	if err != nil {
		return nil, meInternal(err, "checking username uniqueness")
	}
	if usernameTaken {
		return nil, meUsernameTaken(userID, req.Username)
	}

	emailTaken, err := s.orm.User.Query().
		Where(user.EmailEQ(req.Email), user.IDNEQ(userID)).
		Exist(ctx)
	if err != nil {
		return nil, meInternal(err, "checking email uniqueness")
	}
	if emailTaken {
		return nil, meEmailTaken(userID, req.Email)
	}

	u, err := s.orm.User.UpdateOneID(userID).
		SetUsername(req.Username).
		SetEmail(req.Email).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			if taken, checkErr := s.orm.User.Query().Where(user.UsernameEQ(req.Username), user.IDNEQ(userID)).Exist(ctx); checkErr == nil && taken {
				return nil, meUsernameTaken(userID, req.Username)
			}
			if taken, checkErr := s.orm.User.Query().Where(user.EmailEQ(req.Email), user.IDNEQ(userID)).Exist(ctx); checkErr == nil && taken {
				return nil, meEmailTaken(userID, req.Email)
			}
		}
		return nil, meInternal(err, "replacing user profile")
	}
	return u, nil
}

// ChangePassword 修改已登录用户的密码（自定义子资源动作）。
func (s *Me) ChangePassword(ctx context.Context, userID string, req *api.ChangePasswordRequest) (err error) {
	u, err := s.orm.User.Query().
		Where(user.IDEQ(userID)).
		First(ctx)
	if err != nil {
		return meInternal(err, "fetching user for password change")
	}

	if err := verifyPassword(u.PasswordHash, req.OldPassword); err != nil {
		slog.WarnContext(ctx, "invalid old password", "user_id", userID)
		return errMe.
			Code(codes.MeInvalidPassword).
			User(userID).
			With(rest.MetaKey, rest.PreconditionInfo{
				Type:    "INVALID_PASSWORD",
				Subject: "old_password",
			}).
			New("invalid old password")
	}

	newHash, err := hashPassword(req.NewPassword, s.bcryptCost)
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
