package handler

import (
	"context"

	"github.com/samber/do/v2"

	"github.com/liukeshao/go-rest-template/ent"
	"github.com/liukeshao/go-rest-template/internal/service"
	"github.com/liukeshao/go-rest-template/pkg/api"
)

var _ api.BooksHandler = (*Books)(nil)

type Books struct {
	book *service.Book
}

func NewBooks(i do.Injector) (*Books, error) {
	return &Books{
		book: do.MustInvoke[*service.Book](i),
	}, nil
}

func (h *Books) CreateBook(ctx context.Context, req *api.CreateBookRequest) (*api.Book, error) {
	b, err := h.book.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	res := toBook(b)
	return &res, nil
}

func (h *Books) ListBooks(ctx context.Context, req *api.ListBooksRequest) (*api.BookList, error) {
	res, err := h.book.List(ctx, req)
	if err != nil {
		return nil, err
	}
	return &api.BookList{
		Items:     mapSlice(res.Items, toBook),
		Page:      res.Page,
		PageSize:  res.PageSize,
		TotalSize: res.TotalSize,
	}, nil
}

func (h *Books) GetBook(ctx context.Context, req *api.GetBookRequest) (*api.Book, error) {
	b, err := h.book.Get(ctx, req.BookID)
	if err != nil {
		return nil, err
	}
	res := toBook(b)
	return &res, nil
}

func (h *Books) ReplaceBook(ctx context.Context, req *api.ReplaceBookRequest) (*api.Book, error) {
	b, err := h.book.Replace(ctx, req)
	if err != nil {
		return nil, err
	}
	res := toBook(b)
	return &res, nil
}

func (h *Books) DeleteBook(ctx context.Context, req *api.DeleteBookRequest) error {
	return h.book.Delete(ctx, req.BookID)
}

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
