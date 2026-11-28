package service

import (
	"context"
	"strings"

	"github.com/samber/do/v2"
	"github.com/samber/oops"

	"github.com/liukeshao/go-rest-template/ent"
	"github.com/liukeshao/go-rest-template/ent/predicate"
	entuser "github.com/liukeshao/go-rest-template/ent/user"
	"github.com/liukeshao/go-rest-template/internal/database"
	"github.com/liukeshao/go-rest-template/pkg/httperr"
)

var errUser = oops.In("user")

// ReplaceUserInput 描述更新用户资料的领域输入，避免通用 User 服务耦合特定路由的 Request DTO。
type ReplaceUserInput struct {
	Username string
	Email    string
}

type User struct {
	orm *ent.Client
}

func NewUser(i do.Injector) (*User, error) {
	return &User{
		orm: do.MustInvoke[*database.ORM](i).Client,
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
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

func userFieldTaken(domain oops.OopsErrorBuilder, actorID, field, value string) error {
	b := domain
	if actorID != "" {
		b = b.User(actorID)
	}
	return b.
		With(httperr.MetaKey, httperr.ResourceInfo{
			ResourceType: "user",
			ResourceName: value,
			Field:        field,
		}).
		Wrap(httperr.Conflict)
}

// checkUserUnique 检查 email 与 username 是否被其他用户占用；wrapErr 为 nil 时忽略查询错误（用于约束冲突回查）。
func checkUserUnique(
	ctx context.Context,
	orm *ent.Client,
	domain oops.OopsErrorBuilder,
	excludeUserID, email, username string,
	wrapErr func(error, string) error,
) error {
	checkField := func(field, value string, pred predicate.User) error {
		if value == "" {
			return nil
		}
		q := orm.User.Query().Where(pred)
		if excludeUserID != "" {
			q = q.Where(entuser.IDNEQ(excludeUserID))
		}
		exists, err := q.Exist(ctx)
		if err != nil {
			if wrapErr != nil {
				return wrapErr(err, "checking "+field+" uniqueness")
			}
			return nil
		}
		if exists {
			return userFieldTaken(domain, excludeUserID, field, value)
		}
		return nil
	}

	if err := checkField("email", email, entuser.EmailEQ(email)); err != nil {
		return err
	}
	return checkField("username", username, entuser.UsernameEQ(username))
}

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

func (s *User) Replace(ctx context.Context, userID string, in ReplaceUserInput) (*ent.User, error) {
	email := normalizeEmail(in.Email)
	if err := checkUserUnique(ctx, s.orm, errUser, userID, email, in.Username, userInternal); err != nil {
		return nil, err
	}

	u, err := s.orm.User.UpdateOneID(userID).
		SetUsername(in.Username).
		SetEmail(email).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, userNotFound(errUser, userID)
		}
		if ent.IsConstraintError(err) {
			if conflictErr := checkUserUnique(ctx, s.orm, errUser, userID, email, in.Username, nil); conflictErr != nil {
				return nil, conflictErr
			}
		}
		return nil, userInternal(err, "replacing user profile")
	}
	return u, nil
}
