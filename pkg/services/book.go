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
	"github.com/liukeshao/echo-template/pkg/validator"
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

// bookAlreadyExists 构造 BOOK_ALREADY_EXISTS oops 错误（HTTP 409）。
func bookAlreadyExists(id string) error {
	return oops.
		Code(codes.BookAlreadyExists.String()).
		With(codes.CtxKeyParams, map[string]string{"book_id": id}).
		Public(fmt.Sprintf("Book %q already exists. Please use a different book_id.", id)).
		Errorf("book already exists")
}

// bookNotFound 构造 BOOK_NOT_FOUND oops 错误（HTTP 404）。
// Errorf message 保持低基数（无变量），供日志与 APM 聚合分组。
func bookNotFound(id string) error {
	return oops.
		Code(codes.BookNotFound.String()).
		With(codes.CtxKeyParams, map[string]string{"book_id": id}).
		Public(fmt.Sprintf("Book %q not found.", id)).
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

// Create 创建书籍。
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

// List 获取书籍列表，支持分页、排序和过滤。
// 返回值依次为：当前页书籍、总数量、错误；分页参数由调用方（handler）负责解析。
func (s *BookService) List(ctx context.Context, in metav1.ListOptions) ([]*ent.Book, int32, error) {
	orderOpts, err := parseBookOrderBy(in.OrderBy)
	if err != nil {
		return nil, 0, err
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
	if total == 0 {
		return []*ent.Book{}, 0, nil
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

// Replace 全量替换书籍 —— Kubernetes 风格的幂等 PUT 语义。
//
// 行为：
//   - 必填字段（title / author）直接 Set
//   - 可选字段（description / price / published_at）有则 Set，无则 Clear
//   - id / created_at 不会被覆盖；updated_at 由 ent hook 自动刷新
//
// PATCH handler 通过"GET → apply patch → Replace"流程复用本方法，
// 因此 patch 的所有协议（RFC 6902 / 7386 / Shorthand）最终统一落到这一处幂等写入。
func (s *BookService) Replace(ctx context.Context, id string, req *types.ReplaceBookRequest) (*ent.Book, error) {
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
func (s *BookService) Delete(ctx context.Context, id string) error {
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
			v.Add("order_by", validator.KeyInvalidOrderByClause, map[string]string{
				"clause": part,
			})
			continue
		}

		fieldName := strings.ToLower(tokens[0])
		desc := len(tokens) == 2 && strings.ToLower(tokens[1]) == "desc"
		if len(tokens) == 2 && !desc {
			v.Add("order_by", validator.KeyInvalidOrderByDirection, map[string]string{
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
			v.Add("order_by", validator.KeyUnknownOrderByField, map[string]string{
				"field":   fieldName,
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
