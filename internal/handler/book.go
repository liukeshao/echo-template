package handler

import (
	"context"

	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/internal/service"
	"github.com/liukeshao/echo-template/pkg/api"
)

var _ api.BooksHandler = (*Book)(nil)

type Book struct {
	book *service.Book
}

func NewBook(i do.Injector) (*Book, error) {
	return &Book{
		book: do.MustInvoke[*service.Book](i),
	}, nil
}

func (h *Book) CreateBook(ctx context.Context, req *api.CreateBookRequest) (*api.Book, error) {
	b, err := h.book.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	res := toBook(b)
	return &res, nil
}

func (h *Book) ListBooks(ctx context.Context, req *api.ListBooksRequest) (*api.BookList, error) {
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

func (h *Book) GetBook(ctx context.Context, req *api.GetBookRequest) (*api.Book, error) {
	b, err := h.book.Get(ctx, req.Book)
	if err != nil {
		return nil, err
	}
	res := toBook(b)
	return &res, nil
}

func (h *Book) ReplaceBook(ctx context.Context, req *api.ReplaceBookRequest) (*api.Book, error) {
	b, err := h.book.Replace(ctx, req.Book, req)
	if err != nil {
		return nil, err
	}
	res := toBook(b)
	return &res, nil
}

func (h *Book) DeleteBook(ctx context.Context, req *api.DeleteBookRequest) error {
	return h.book.Delete(ctx, req.Book)
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
