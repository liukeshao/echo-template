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
	"github.com/liukeshao/echo-template/internal/database"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/httperr"
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
		With("op", op).
		Join(httperr.Internal, err)
}

// Replace 全量替换当前用户的可写资料（username / email），Kubernetes 风格的幂等 PUT。
func (s *Me) Replace(ctx context.Context, userID string, req *api.ReplaceMeRequest) (*ent.User, error) {
	if err := checkUserUnique(ctx, s.orm, errMe, userID, req.Email, req.Username, meInternal); err != nil {
		return nil, err
	}

	u, err := s.orm.User.UpdateOneID(userID).
		SetUsername(req.Username).
		SetEmail(req.Email).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			if conflictErr := checkUserUnique(ctx, s.orm, errMe, userID, req.Email, req.Username, nil); conflictErr != nil {
				return nil, conflictErr
			}
		}
		return nil, meInternal(err, "replacing user profile")
	}
	return u, nil
}

// ChangePassword 修改已登录用户的密码（自定义子资源动作）。
func (s *Me) ChangePassword(ctx context.Context, userID string, req *api.ChangePasswordRequest) error {
	u, err := s.orm.User.Query().
		Where(user.IDEQ(userID)).
		First(ctx)
	if err != nil {
		return meInternal(err, "fetching user for password change")
	}

	if err := verifyPassword(u.PasswordHash, req.OldPassword); err != nil {
		slog.WarnContext(ctx, "invalid old password", "user_id", userID)
		return errMe.
			User(userID).
			With(httperr.MetaKey, httperr.PreconditionInfo{
				Type:    httperr.PreconditionInvalidPassword,
				Subject: "old_password",
			}).
			Wrap(httperr.BadRequest)
	}

	newHash, err := hashPassword(req.NewPassword, s.bcryptCost)
	if err != nil {
		return meInternal(err, "hashing new password")
	}

	return withTx(ctx, s.orm, meInternal, func(tx *ent.Tx) error {
		if err := tx.User.UpdateOneID(userID).SetPasswordHash(newHash).Exec(ctx); err != nil {
			return meInternal(err, "updating password")
		}
		if _, err := tx.Token.Update().
			Where(token.UserID(userID), token.IsRevoked(false)).
			SetIsRevoked(true).
			Save(ctx); err != nil {
			return meInternal(err, "revoking tokens after password change")
		}
		return nil
	})
}
