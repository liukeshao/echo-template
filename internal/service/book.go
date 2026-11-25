package service

import (
	"context"
	"strings"
	"uuid"

	"entgo.io/ent/dialect/sql"
	"github.com/samber/do/v2"
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/ent/book"
	"github.com/liukeshao/echo-template/ent/predicate"
	"github.com/liukeshao/echo-template/internal/codes"
	"github.com/liukeshao/echo-template/internal/database"
	"github.com/liukeshao/echo-template/pkg/api"
	"github.com/liukeshao/echo-template/pkg/rest"
	"github.com/liukeshao/echo-template/pkg/validator"
)

var errBook = oops.In("book")

// Book 书籍服务
type Book struct {
	orm *ent.Client
}

// NewBook 构造 Book 并从 DI 容器注入依赖。
func NewBook(i do.Injector) (*Book, error) {
	return &Book{
		orm: do.MustInvoke[*database.ORM](i).Client,
	}, nil
}

// bookAlreadyExists 构造 BOOK_ALREADY_EXISTS oops 错误（HTTP 409）。
func bookAlreadyExists(id string) error {
	return errBook.
		Code(codes.BookAlreadyExists).
		With(rest.MetaKey, rest.ResourceInfo{
			ResourceType: "book",
			ResourceName: id,
			Field:        "book_id",
		}).
		New("book already exists")
}

// bookNotFound 构造 BOOK_NOT_FOUND oops 错误（HTTP 404）。
func bookNotFound(id string) error {
	return errBook.
		Code(codes.BookNotFound).
		With(rest.MetaKey, rest.ResourceInfo{
			ResourceType: "book",
			ResourceName: id,
		}).
		New("book not found")
}

// bookDB 构造携带 database 标签的 BOOK_INTERNAL_ERROR oops 错误（内部上下文仅进日志）。
func bookDB(err error, op, id string) error {
	return errBook.
		Tags("database").
		Code(codes.BookInternalError).
		With("book_id", id).
		Wrapf(err, "%s", op)
}

// Create 创建书籍。
//
// 若 req.BookID 非空则使用该值作为资源 ID；否则自动生成 UUID v7。
// 若指定的 ID 已存在，返回 BOOK_ALREADY_EXISTS 错误。
func (s *Book) Create(ctx context.Context, req *api.CreateBookRequest) (*ent.Book, error) {
	id := req.BookID
	if id == "" {
		id = uuid.NewV7().String()
	}

	q := s.orm.Book.Create().
		SetID(id).
		SetTitle(req.Title).
		SetAuthor(req.Author)

	if strings.TrimSpace(req.Description) != "" {
		q.SetDescription(req.Description)
	}
	if req.Price != nil {
		q.SetPrice(*req.Price)
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
func (s *Book) Get(ctx context.Context, id string) (*ent.Book, error) {
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

// List 获取书籍列表，支持分页、排序和按请求字段独立过滤。
func (s *Book) List(ctx context.Context, req *api.ListBooksRequest) (*Page[*ent.Book], error) {
	if req == nil {
		req = &api.ListBooksRequest{}
	}

	offset, limit, page := resolvePagination(req.Page, req.PageSize, defaultPageSize, maxPageSize)

	orderOpts, err := parseBookOrderBy(req.OrderBy)
	if err != nil {
		return nil, err
	}

	preds := buildBookPredicates(req)

	total, err := s.orm.Book.Query().Where(preds...).Count(ctx)
	if err != nil {
		return nil, bookDB(err, "counting books", "")
	}
	if total == 0 {
		return &Page[*ent.Book]{
			Items:     []*ent.Book{},
			Page:      page,
			PageSize:  limit,
			TotalSize: 0,
		}, nil
	}

	books, err := s.orm.Book.Query().
		Where(preds...).
		Order(orderOpts...).
		Limit(int(limit)).
		Offset(offset).
		All(ctx)
	if err != nil {
		return nil, bookDB(err, "listing books", "")
	}

	return &Page[*ent.Book]{
		Items:     books,
		Page:      page,
		PageSize:  limit,
		TotalSize: int32(total),
	}, nil
}

// buildBookPredicates 根据 ListBooksRequest 的显式字段构造 ent 查询条件。
func buildBookPredicates(req *api.ListBooksRequest) []predicate.Book {
	if req == nil {
		return nil
	}
	var preds []predicate.Book
	if title := strings.TrimSpace(req.Title); title != "" {
		preds = append(preds, book.TitleContainsFold(title))
	}
	if author := strings.TrimSpace(req.Author); author != "" {
		preds = append(preds, book.AuthorContainsFold(author))
	}
	if req.MinPrice != nil {
		preds = append(preds, book.PriceGTE(*req.MinPrice))
	}
	if req.MaxPrice != nil {
		preds = append(preds, book.PriceLTE(*req.MaxPrice))
	}
	return preds
}

// Replace 全量替换书籍 —— Kubernetes 风格的幂等 PUT 语义。
func (s *Book) Replace(ctx context.Context, id string, req *api.ReplaceBookRequest) (*ent.Book, error) {
	q := s.orm.Book.UpdateOneID(id).
		SetTitle(req.Title).
		SetAuthor(req.Author)

	if strings.TrimSpace(req.Description) != "" {
		q.SetDescription(req.Description)
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

	b, err := q.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, bookNotFound(id)
		}
		return nil, bookDB(err, "replacing book", id)
	}
	return b, nil
}

// Delete 删除书籍（逻辑删除）。
func (s *Book) Delete(ctx context.Context, id string) error {
	if err := s.orm.Book.DeleteOneID(id).Exec(ctx); err != nil {
		if ent.IsNotFound(err) {
			return bookNotFound(id)
		}
		return bookDB(err, "deleting book", id)
	}
	return nil
}

// --- 排序辅助 ---

const bookAllowedOrderByFields = "title, author, price, created_at, updated_at, published_at"

// parseBookOrderBy 将 order_by 字符串解析为 ent OrderOption 列表。
// 格式：逗号分隔的 "field" 或 "field desc"；默认 created_at desc。
// 支持字段：title, author, price, created_at, updated_at, published_at。
func parseBookOrderBy(orderBy string) ([]book.OrderOption, error) {
	if strings.TrimSpace(orderBy) == "" {
		return []book.OrderOption{book.ByCreatedAt(sql.OrderDesc())}, nil
	}

	var (
		opts []book.OrderOption
		v    validator.FieldViolations
	)
	for part := range strings.SplitSeq(orderBy, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		tokens := strings.Fields(part)
		if len(tokens) > 2 {
			v.Add("order_by", validator.CodeInvalidOrderByClause, map[string]string{
				"clause": part,
			})
			continue
		}

		fieldName := strings.ToLower(tokens[0])
		desc := len(tokens) == 2 && strings.ToLower(tokens[1]) == "desc"
		if len(tokens) == 2 && !desc {
			v.Add("order_by", validator.CodeInvalidOrderByDirection, map[string]string{
				"direction": tokens[1],
				"clause":    part,
			})
			continue
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
			v.Add("order_by", validator.CodeUnknownOrderByField, map[string]string{
				"value":   fieldName,
				"allowed": bookAllowedOrderByFields,
			})
		}
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	if len(opts) == 0 {
		return []book.OrderOption{book.ByCreatedAt(sql.OrderDesc())}, nil
	}
	return opts, nil
}
