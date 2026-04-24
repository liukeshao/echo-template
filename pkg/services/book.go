package services

import (
	"context"
	"fmt"
	"strings"

	"entgo.io/ent/dialect/sql"
	"github.com/samber/do/v2"
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/book"
	"github.com/liukeshao/echo-template/pkg/codes"
	"github.com/liukeshao/echo-template/pkg/filter"
	"github.com/liukeshao/echo-template/pkg/infra"
	metav1 "github.com/liukeshao/echo-template/pkg/meta/v1"
	"github.com/liukeshao/echo-template/pkg/types"
	"github.com/liukeshao/echo-template/pkg/utils"
	"github.com/liukeshao/echo-template/transport/rpc/errdetails"
)

// BookService 书籍服务
type BookService struct {
	orm *ent.Client
}

// NewBookService 构造 BookService 并从 DI 容器注入依赖。
func NewBookService(i do.Injector) (*BookService, error) {
	return &BookService{
		orm: do.MustInvoke[*infra.ORM](i).Client,
	}, nil
}

// bookAlreadyExists 构造携带 ResourceInfo 的 BOOK_ALREADY_EXISTS oops 错误（AIP-133）。
func bookAlreadyExists(id string) error {
	return oops.
		Code(codes.BookAlreadyExists.String()).
		With(codes.CtxKeyResourceInfo, errdetails.ResourceInfo{
			Type:         errdetails.TypeResourceInfo,
			ResourceType: "Book",
			ResourceName: id,
		}).
		With(codes.CtxKeyMetadata, map[string]string{
			"book_id": id,
		}).
		Public(fmt.Sprintf("Book %q already exists. Please use a different book_id.", id)).
		Errorf("book already exists")
}

// bookNotFound 构造携带 ResourceInfo 和 ErrorInfo.Metadata 的 BOOK_NOT_FOUND oops 错误。
//
// Public message 作为 AIP-193 response.error.message，包含 id 供调用方快速定位。
// Errorf message 保持低基数（无变量），供 APM 工具聚合分组。
func bookNotFound(id string) error {
	return oops.
		Code(codes.BookNotFound.String()).
		With(codes.CtxKeyResourceInfo, errdetails.ResourceInfo{
			Type:         errdetails.TypeResourceInfo,
			ResourceType: "Book",
			ResourceName: id,
		}).
		With(codes.CtxKeyMetadata, map[string]string{
			"book_id": id,
		}).
		Public(fmt.Sprintf("Book %q not found. Please verify the book ID and try again.", id)).
		Errorf("book not found")
}

// bookDB 构造携带 database 标签的 BOOK_INTERNAL_ERROR oops 错误。
func bookDB(err error, op, id string) error {
	return oops.
		Code(codes.BookInternal.String()).
		Tags("database").
		With("book_id", id).
		Wrapf(err, "%s", op)
}

// Create 创建书籍（AIP-133）。
//
// 若 req.BookID 非空则使用该值作为资源 ID；否则自动生成 ULID。
// 若指定的 ID 已存在，返回 BOOK_ALREADY_EXISTS 错误。
func (s *BookService) Create(ctx context.Context, req *types.CreateBookRequest) (*ent.Book, error) {
	id := req.BookID
	if id == "" {
		id = utils.GenerateULID()
	}

	q := s.orm.Book.Create().
		SetID(id).
		SetTitle(req.Title).
		SetAuthor(req.Author)

	if req.Description != "" {
		q.SetDescription(req.Description)
	}
	if req.Price != 0 {
		q.SetPrice(req.Price)
	}
	if req.PublishedAt != nil {
		q.SetPublishedAt(*req.PublishedAt)
	}

	b, err := q.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, bookAlreadyExists(id)
		}
		return nil, bookDB(err, "creating book", id)
	}
	return b, nil
}

// Get 获取书籍
func (s *BookService) Get(ctx context.Context, id string) (*ent.Book, error) {
	b, err := s.orm.Book.Query().
		Where(book.IDEQ(id)).
		First(ctx)
	switch {
	case err == nil:
	case ent.IsNotFound(err):
		return nil, bookNotFound(id)
	default:
		return nil, bookDB(err, "getting book", id)
	}
	return b, nil
}

// List 获取书籍列表，支持分页、排序和过滤（AIP-132）。
// 返回值依次为：当前页书籍、总数量、错误；分页游标由调用方（handler）负责编解码。
func (s *BookService) List(ctx context.Context, in metav1.ListOptions) ([]*ent.Book, int32, error) {
	orderOpts, err := parseBookOrderBy(in.OrderBy)
	if err != nil {
		return nil, 0, oops.
			Code(codes.ValidationInvalid.String()).
			Public(err.Error()).
			Errorf("invalid order_by: %v", err)
	}

	filterExpr, err := filter.Parse(in.Filter)
	if err != nil {
		return nil, 0, oops.
			Code(codes.ValidationInvalid.String()).
			Public(err.Error()).
			Errorf("invalid filter: %v", err)
	}
	filterPred, err := translateBookFilter(filterExpr)
	if err != nil {
		return nil, 0, oops.
			Code(codes.ValidationInvalid.String()).
			Public(err.Error()).
			Errorf("translate filter: %v", err)
	}

	countQ := s.orm.Book.Query()
	if filterPred != nil {
		countQ = countQ.Where(filterPred)
	}
	total, err := countQ.Count(ctx)
	if err != nil {
		return nil, 0, bookDB(err, "counting books", "")
	}

	listQ := s.orm.Book.Query()
	if filterPred != nil {
		listQ = listQ.Where(filterPred)
	}
	books, err := listQ.
		Order(orderOpts...).
		Limit(int(in.Limit)).
		Offset(int(in.Offset)).
		All(ctx)
	if err != nil {
		return nil, 0, bookDB(err, "listing books", "")
	}

	return books, int32(total), nil
}

// Update 更新书籍（AIP-134）。
//
// field mask 语义：
//   - req.UpdateMask 为空 → 更新请求体中所有非 nil 字段
//   - req.UpdateMask == ["*"] → 全量替换：必填字段设置，可选字段未提供则清除
//   - req.UpdateMask 含具体字段名 → 仅更新 mask 中列出的字段；
//     可选字段在 mask 中但请求体为 nil 时被清除
func (s *BookService) Update(ctx context.Context, id string, req *types.UpdateBookRequest) (*ent.Book, error) {
	_, err := s.orm.Book.Query().
		Where(book.IDEQ(id)).
		First(ctx)
	switch {
	case err == nil:
	case ent.IsNotFound(err):
		return nil, bookNotFound(id)
	default:
		return nil, bookDB(err, "getting book for update", id)
	}

	q := s.orm.Book.UpdateOneID(id)
	applyBookUpdateMask(q, req)

	b, err := q.Save(ctx)
	if err != nil {
		return nil, bookDB(err, "updating book", id)
	}
	return b, nil
}

// applyBookUpdateMask 根据 update_mask 将 req 中的字段变更应用到 ent 更新构造器（AIP-134）。
func applyBookUpdateMask(q *ent.BookUpdateOne, req *types.UpdateBookRequest) {
	mask := req.UpdateMask

	// mask 省略：更新所有已填充（非 nil）的字段
	if len(mask) == 0 {
		if req.Title != nil {
			q.SetTitle(*req.Title)
		}
		if req.Author != nil {
			q.SetAuthor(*req.Author)
		}
		if req.Description != nil {
			q.SetDescription(*req.Description)
		}
		if req.Price != nil {
			q.SetPrice(*req.Price)
		}
		if req.PublishedAt != nil {
			q.SetPublishedAt(*req.PublishedAt)
		}
		return
	}

	// mask=* 全量替换：必填字段直接设置，可选字段未提供则清除
	for _, f := range mask {
		if f == "*" {
			q.SetTitle(*req.Title)   // Validate() 已保证非 nil
			q.SetAuthor(*req.Author) // Validate() 已保证非 nil
			if req.Description != nil {
				q.SetDescription(*req.Description)
			} else {
				q.ClearDescription()
			}
			if req.Price != nil {
				q.SetPrice(*req.Price)
			} else {
				q.ClearPrice()
			}
			if req.PublishedAt != nil {
				q.SetPublishedAt(*req.PublishedAt)
			} else {
				q.ClearPublishedAt()
			}
			return
		}
	}

	// 具体字段 mask：仅操作 mask 中列出的字段
	maskSet := make(map[string]struct{}, len(mask))
	for _, f := range mask {
		maskSet[f] = struct{}{}
	}

	if _, ok := maskSet["title"]; ok && req.Title != nil {
		q.SetTitle(*req.Title)
	}
	if _, ok := maskSet["author"]; ok && req.Author != nil {
		q.SetAuthor(*req.Author)
	}
	if _, ok := maskSet["description"]; ok {
		if req.Description != nil {
			q.SetDescription(*req.Description)
		} else {
			q.ClearDescription()
		}
	}
	if _, ok := maskSet["price"]; ok {
		if req.Price != nil {
			q.SetPrice(*req.Price)
		} else {
			q.ClearPrice()
		}
	}
	if _, ok := maskSet["published_at"]; ok {
		if req.PublishedAt != nil {
			q.SetPublishedAt(*req.PublishedAt)
		} else {
			q.ClearPublishedAt()
		}
	}
}

// Delete 删除书籍
func (s *BookService) Delete(ctx context.Context, id string) error {
	_, err := s.orm.Book.Query().
		Where(book.IDEQ(id)).
		First(ctx)
	switch {
	case err == nil:
	case ent.IsNotFound(err):
		return bookNotFound(id)
	default:
		return bookDB(err, "getting book for delete", id)
	}

	if err := s.orm.Book.DeleteOneID(id).Exec(ctx); err != nil {
		return bookDB(err, "deleting book", id)
	}
	return nil
}

// --- 排序辅助 ---

// parseBookOrderBy 将 order_by 字符串解析为 ent OrderOption 列表。
// 格式：逗号分隔的 "field" 或 "field desc"；默认 created_at desc。
// 支持字段：title, author, price, created_at, updated_at, published_at。
func parseBookOrderBy(orderBy string) ([]book.OrderOption, error) {
	if strings.TrimSpace(orderBy) == "" {
		return []book.OrderOption{book.ByCreatedAt(sql.OrderDesc())}, nil
	}

	var opts []book.OrderOption
	for _, part := range strings.Split(orderBy, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		tokens := strings.Fields(part)
		if len(tokens) > 2 {
			return nil, fmt.Errorf("invalid order_by clause %q; expected \"field\" or \"field desc\"", part)
		}

		fieldName := strings.ToLower(tokens[0])
		desc := len(tokens) == 2 && strings.ToLower(tokens[1]) == "desc"
		if len(tokens) == 2 && !desc {
			return nil, fmt.Errorf("invalid order_by direction %q in %q; use \"desc\" or omit for ascending", tokens[1], part)
		}

		dir := sql.OrderAsc()
		if desc {
			dir = sql.OrderDesc()
		}

		switch fieldName {
		case "title":
			opts = append(opts, book.ByTitle(dir))
		case "author":
			opts = append(opts, book.ByAuthor(dir))
		case "price":
			opts = append(opts, book.ByPrice(dir))
		case "created_at":
			opts = append(opts, book.ByCreatedAt(dir))
		case "updated_at":
			opts = append(opts, book.ByUpdatedAt(dir))
		case "published_at":
			opts = append(opts, book.ByPublishedAt(dir))
		default:
			return nil, fmt.Errorf("unknown order_by field %q; supported: title, author, price, created_at, updated_at, published_at", fieldName)
		}
	}
	return opts, nil
}
