package service

import (
	"context"

	"github.com/liukeshao/go-rest-template/ent"
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
