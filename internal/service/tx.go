package service

import (
	"context"

	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/ent"
	userEnt "github.com/liukeshao/echo-template/ent/user"
	"github.com/liukeshao/echo-template/pkg/httperr"
)

// withTx 在单个数据库事务中执行 fn，并保证任何错误或 panic 都会回滚事务（避免 SQLite 单连接写锁泄漏）。
// wrapErr 用于将开启或提交事务失败的底层错误包装为对应领域的 5xx oops 错误。
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

// userFieldTaken 构造用户唯一性字段（email / username）冲突的 409 Conflict oops 错误。
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

// checkUserUnique 检查 email 与 username 是否被（除 excludeUserID 外的）其他用户占用。
// 若数据库查询失败：当 wrapErr 非空时立即包装为 5xx 返回；当 wrapErr 为 nil（用于约束冲突后回查）时忽略查询失败。
func checkUserUnique(
	ctx context.Context,
	orm *ent.Client,
	domain oops.OopsErrorBuilder,
	excludeUserID, email, username string,
	wrapErr func(error, string) error,
) error {
	if email != "" {
		q := orm.User.Query().Where(userEnt.EmailEQ(email))
		if excludeUserID != "" {
			q = q.Where(userEnt.IDNEQ(excludeUserID))
		}
		exists, err := q.Exist(ctx)
		if err != nil {
			if wrapErr != nil {
				return wrapErr(err, "checking email uniqueness")
			}
		} else if exists {
			return userFieldTaken(domain, excludeUserID, "email", email)
		}
	}
	if username != "" {
		q := orm.User.Query().Where(userEnt.UsernameEQ(username))
		if excludeUserID != "" {
			q = q.Where(userEnt.IDNEQ(excludeUserID))
		}
		exists, err := q.Exist(ctx)
		if err != nil {
			if wrapErr != nil {
				return wrapErr(err, "checking username uniqueness")
			}
		} else if exists {
			return userFieldTaken(domain, excludeUserID, "username", username)
		}
	}
	return nil
}
