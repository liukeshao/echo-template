package service

import (
	"context"

	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/predicate"
	entuser "github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/pkg/httperr"
)

// withTx 在单事务内执行 fn，确保任何错误或 panic 都会回滚（避免 SQLite 单连接写锁泄漏）。
func withTx(
	ctx context.Context,
	orm *ent.Client,
	wrapErr func(err error, op string) error,
	fn func(tx *ent.Tx) error,
) error {
	tx, err := orm.Tx(ctx)
	if err != nil {
		return wrapErr(err, "beginning transaction")
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return wrapErr(err, "committing transaction")
	}
	committed = true
	return nil
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
