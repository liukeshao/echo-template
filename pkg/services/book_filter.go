package services

import (
	"github.com/liukeshao/echo-template/ent/book"
	"github.com/liukeshao/echo-template/ent/predicate"
	"github.com/liukeshao/echo-template/pkg/filter"
)

// bookFilter 是书籍领域的 AIP-160 过滤翻译器。
// 在包初始化时构造，不可变，并发安全。
//
// 支持字段：
//
//	title  (string)  = != < <= > >= :
//	author (string)  = != < <= > >= :
//	price  (float64) = != < <= > >=
var bookFilter = filter.NewTranslator(book.And, book.Or, book.Not).
	String("title",
		book.TitleEQ, book.TitleNEQ,
		book.TitleLT, book.TitleLTE, book.TitleGT, book.TitleGTE,
		book.TitleContainsFold, book.TitleHasPrefix, book.TitleHasSuffix,
	).
	String("author",
		book.AuthorEQ, book.AuthorNEQ,
		book.AuthorLT, book.AuthorLTE, book.AuthorGT, book.AuthorGTE,
		book.AuthorContainsFold, book.AuthorHasPrefix, book.AuthorHasSuffix,
	).
	Float64("price",
		book.PriceEQ, book.PriceNEQ,
		book.PriceLT, book.PriceLTE, book.PriceGT, book.PriceGTE,
	)

// translateBookFilter 将 filter.Expr AST 翻译为 ent predicate.Book。
func translateBookFilter(expr filter.Expr) (predicate.Book, error) {
	return bookFilter.Translate(expr)
}
