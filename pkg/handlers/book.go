package handlers

import (
	"context"
	"net/http"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/httpruntime"
	"github.com/liukeshao/echo-template/pkg/services"
)

var _ api.BooksHandler = (*BookHandler)(nil)

// BookHandler 书籍处理器（实现 TypeSpec 生成的 api.BooksHandler 接口）。
// 负责多服务编排与 *ent.Book → api.Book 投影；
// PATCH /api/v1/books/{book} 由 TypeSpec @autoPatch(Books.get, Books.replace) 自动串联。
type BookHandler struct {
	book *services.BookService
}

// NewBookHandler 构造 BookHandler 并注入依赖。
func NewBookHandler(i do.Injector) (*BookHandler, error) {
	return &BookHandler{
		book: do.MustInvoke[*services.BookService](i),
	}, nil
}

// Mount 将 /api/v1/books 路由挂载到标准 *http.ServeMux。
func (h *BookHandler) Mount(mux *http.ServeMux, rt *httpruntime.Runtime, auth func(http.Handler) http.Handler) {
	api.MountBooksHandler(mux, h, rt, auth)
}

func (h *BookHandler) CreateBook(ctx context.Context, req *api.CreateBookRequest) (*api.Book, error) {
	b, err := h.book.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	res := toBook(b)
	return &res, nil
}

func (h *BookHandler) ListBooks(ctx context.Context, req *api.ListBooksRequest) (*api.BookList, error) {
	opts, page := resolveListOptions(req.Page, req.PageSize, req.OrderBy, req.Filter, defaultPageSize, maxPageSize)
	books, total, err := h.book.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &api.BookList{
		Items:     mapSlice(books, toBook),
		Page:      page,
		PageSize:  opts.Limit,
		TotalSize: total,
	}, nil
}

func (h *BookHandler) GetBook(ctx context.Context, req *api.GetBookRequest) (*api.Book, error) {
	b, err := h.book.Get(ctx, req.Book)
	if err != nil {
		return nil, err
	}
	res := toBook(b)
	return &res, nil
}

func (h *BookHandler) ReplaceBook(ctx context.Context, req *api.ReplaceBookRequest) (*api.Book, error) {
	b, err := h.book.Replace(ctx, req.Book, req)
	if err != nil {
		return nil, err
	}
	res := toBook(b)
	return &res, nil
}

func (h *BookHandler) DeleteBook(ctx context.Context, req *api.DeleteBookRequest) error {
	return h.book.Delete(ctx, req.Book)
}

// toBook 将 *ent.Book 映射为对外资源视图 api.Book，隐藏 deleted_at 等内部字段。
func toBook(b *ent.Book) api.Book {
	return api.Book{
		ID:          b.ID,
		Title:       b.Title,
		Author:      b.Author,
		Description: b.Description,
		Price:       b.Price,
		PublishedAt: b.PublishedAt,
		CreatedAt:   b.CreatedAt,
		UpdatedAt:   b.UpdatedAt,
	}
}
