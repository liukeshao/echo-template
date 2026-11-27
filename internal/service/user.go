package service

import (
	"context"

	"github.com/samber/do/v2"
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/ent"
	entuser "github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/internal/database"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/httperr"
)

var errUser = oops.In("user")

// User 用户领域服务（纯用户资料查询与更新，不耦合密码策略或令牌表）。
type User struct {
	orm *ent.Client
}

// NewUser 构造 User 并从 DI 容器注入依赖。
func NewUser(i do.Injector) (*User, error) {
	return &User{
		orm: do.MustInvoke[*database.ORM](i).Client,
	}, nil
}

func userInternal(err error, op string) error {
	return errUser.
		With("op", op).
		Join(httperr.Internal, err)
}

func userNotFound(domain oops.OopsErrorBuilder, userID string) error {
	return domain.
		With(httperr.MetaKey, httperr.ResourceInfo{
			ResourceType: "user",
			ResourceName: userID,
		}).
		Wrap(httperr.NotFound)
}

// Get 按 ID 查询用户资料；不存在时返回 404 Not Found。
func (s *User) Get(ctx context.Context, userID string) (*ent.User, error) {
	u, err := s.orm.User.Query().
		Where(entuser.IDEQ(userID)).
		Only(ctx)
	switch {
	case err == nil:
		return u, nil
	case ent.IsNotFound(err):
		return nil, userNotFound(errUser, userID)
	default:
		return nil, userInternal(err, "getting user")
	}
}

// Replace 全量替换指定用户的可写资料（username / email），Kubernetes 风格的幂等 PUT。
func (s *User) Replace(ctx context.Context, userID string, req *api.ReplaceMeRequest) (*ent.User, error) {
	if err := checkUserUnique(ctx, s.orm, errUser, userID, req.Email, req.Username, userInternal); err != nil {
		return nil, err
	}

	u, err := s.orm.User.UpdateOneID(userID).
		SetUsername(req.Username).
		SetEmail(req.Email).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, userNotFound(errUser, userID)
		}
		if ent.IsConstraintError(err) {
			if conflictErr := checkUserUnique(ctx, s.orm, errUser, userID, req.Email, req.Username, nil); conflictErr != nil {
				return nil, conflictErr
			}
		}
		return nil, userInternal(err, "replacing user profile")
	}
	return u, nil
}
