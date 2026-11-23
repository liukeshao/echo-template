package codes

import "net/http"

// Book 领域错误码。
//
// 命名规范：<领域>_<错误语义>，使用 UPPER_SNAKE_CASE。
const (
	// BookNotFound 表示指定 ID 的书籍不存在。
	BookNotFound Code = "BOOK_NOT_FOUND"

	// BookAlreadyExists 表示使用相同 ID 创建书籍时发生冲突。
	BookAlreadyExists Code = "BOOK_ALREADY_EXISTS"

	// BookInternal 表示书籍操作遭遇内部/数据库错误。
	BookInternal Code = "BOOK_INTERNAL_ERROR"
)

func init() {
	Register(BookNotFound, http.StatusNotFound)
	Register(BookAlreadyExists, http.StatusConflict)
	Register(BookInternal, http.StatusInternalServerError)
}
