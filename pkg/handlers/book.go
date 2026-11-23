package handlers

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/ent"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
)

// BookHandler 书籍处理器
type BookHandler struct {
	book *services.BookService
}

// NewBookHandler 构造 BookHandler 并注入依赖。
func NewBookHandler(i do.Injector) (*BookHandler, error) {
	return &BookHandler{
		book: do.MustInvoke[*services.BookService](i),
	}, nil
}

// Routes 注册路由（RESTful CRUD + 多协议 PATCH）。
//
// 方法语义：
//   - POST   ：创建（非幂等，服务端可生成 ID）
//   - GET    ：读取（单资源或集合）
//   - PUT    ：幂等全量替换；可选字段未提供视为清除
//   - PATCH  ：局部更新；根据 Content-Type 分派 RFC 6902 / 7386 / Shorthand
//   - DELETE ：删除
func (h *BookHandler) Routes(g *echo.Group, auth echo.MiddlewareFunc) {
	r := g.Group("/books", auth)

	r.POST("", h.Create)
	r.GET("", h.List)
	r.GET("/:book", h.Get)
	r.PUT("/:book", h.Replace)
	r.PATCH("/:book", h.Patch)
	r.DELETE("/:book", h.Delete)
}

// toBook 将 *ent.Book 映射为对外资源视图 types.Book，隐藏 deleted_at 等内部字段。
func toBook(b *ent.Book) types.Book {
	return types.Book{
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

// Create 创建书籍
//
//	@Summary		创建书籍
//	@Description	book_id 可选，若指定则用作资源 ID（ULID 格式）；若省略则服务端自动生成。book_id 重复时返回 409。
//	@Tags			books
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		types.CreateBookRequest	true	"书籍信息"
//	@Success		201		{object}	types.Book
//	@Failure		400		{object}	types.Problem
//	@Failure		401		{object}	types.Problem
//	@Failure		409		{object}	types.Problem	"book_id 已存在（BOOK_ALREADY_EXISTS）"
//	@Router			/api/v1/books [post]
func (h *BookHandler) Create(c *echo.Context) error {
	req, err := bindAndValidate[types.CreateBookRequest](c)
	if err != nil {
		return err
	}

	b, err := h.book.Create(c.Request().Context(), &req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, toBook(b))
}

// List 获取书籍列表
//
//	@Summary	获取书籍列表
//	@Tags		books
//	@Security	BearerAuth
//	@Produce	json
//	@Param		page		query		int		false	"页码，从 1 开始（默认 1）"
//	@Param		page_size	query		int		false	"每页数量（默认 50，最大 1000）"
//	@Param		order_by	query		string	false	"排序，如 'price desc,title'；支持 title/author/price/created_at/updated_at/published_at"
//	@Param		filter		query		string	false	"过滤表达式，如 title : 'Go' AND price < 50"
//	@Success	200			{object}	types.BookList
//	@Failure	400			{object}	types.Problem
//	@Failure	401			{object}	types.Problem
//	@Router		/api/v1/books [get]
func (h *BookHandler) List(c *echo.Context) error {
	req, err := bindAndValidate[types.ListRequest](c)
	if err != nil {
		return err
	}

	opts, page := resolveListOptions(req, defaultPageSize, maxPageSize)
	books, total, err := h.book.List(c.Request().Context(), opts)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, types.BookList{
		Items:     mapSlice(books, toBook),
		Page:      page,
		PageSize:  opts.Limit,
		TotalSize: total,
	})
}

// Get 获取书籍详情
//
//	@Summary	获取书籍详情
//	@Tags		books
//	@Security	BearerAuth
//	@Produce	json
//	@Param		book	path		string	true	"书籍资源 ID（ULID）"
//	@Success	200		{object}	types.Book
//	@Failure	401		{object}	types.Problem
//	@Failure	404		{object}	types.Problem
//	@Router		/api/v1/books/{book} [get]
func (h *BookHandler) Get(c *echo.Context) error {
	var path types.BookPath
	if err := c.Bind(&path); err != nil {
		return err
	}

	b, err := h.book.Get(c.Request().Context(), path.Book)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, toBook(b))
}

// Replace 替换书籍（幂等全量更新）。
//
//	@Summary		替换书籍
//	@Description	使用请求体中的全部字段替换资源；未提供的可选字段会被清除。响应为更新后的资源。
//	@Tags			books
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			book	path		string						true	"书籍资源 ID（ULID）"
//	@Param			body	body		types.ReplaceBookRequest	true	"完整书籍信息"
//	@Success		200		{object}	types.Book
//	@Failure		400		{object}	types.Problem
//	@Failure		401		{object}	types.Problem
//	@Failure		404		{object}	types.Problem
//	@Router			/api/v1/books/{book} [put]
func (h *BookHandler) Replace(c *echo.Context) error {
	req, err := bindAndValidate[types.ReplaceBookRequest](c)
	if err != nil {
		return err
	}

	b, err := h.book.Replace(c.Request().Context(), req.Book, &req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, toBook(b))
}

// Patch 局部更新书籍（多协议 PATCH）。
//
// 根据 Content-Type 选择不同 patch 协议；统一通过"GET → apply → Replace"流程落库，
// 因此 PATCH 与 PUT 共享同一份服务端校验与持久化逻辑。
//
//	@Summary		局部更新书籍
//	@Description	根据 Content-Type 选择 patch 协议：application/json-patch+json (RFC 6902) / application/merge-patch+json (RFC 7386) / application/merge-patch+shorthand。
//	@Tags			books
//	@Security		BearerAuth
//	@Accept			application/merge-patch+json,application/json-patch+json,application/merge-patch+shorthand,application/json
//	@Produce		json
//	@Param			book	path		string	true	"书籍资源 ID（ULID）"
//	@Param			body	body		object	true	"patch 主体（具体格式由 Content-Type 决定）"
//	@Success		200		{object}	types.Book
//	@Failure		400		{object}	types.Problem
//	@Failure		401		{object}	types.Problem
//	@Failure		404		{object}	types.Problem
//	@Failure		415		{object}	types.Problem	"不支持的 PATCH Content-Type"
//	@Router			/api/v1/books/{book} [patch]
func (h *BookHandler) Patch(c *echo.Context) error {
	ctx := c.Request().Context()

	// PATCH 不能直接 c.Bind：body 可能是 JSON Patch 数组而不是任何 struct。
	// 仅绑定路径参数，body 留给 decodePatch 独占处理。
	var path types.BookPath
	if err := echo.BindPathValues(c, &path); err != nil {
		return err
	}

	current, err := h.book.Get(ctx, path.Book)
	if err != nil {
		return err
	}

	req, err := decodePatch[types.ReplaceBookRequest](c, toBook(current))
	if err != nil {
		return err
	}
	// 覆盖 patch 中可能伪造的 path param，路径才是权威。
	req.Book = path.Book
	if err := req.Validate(); err != nil {
		return err
	}

	b, err := h.book.Replace(ctx, path.Book, &req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, toBook(b))
}

// Delete 删除书籍
//
//	@Summary		删除书籍
//	@Description	资源不存在时返回 404（BOOK_NOT_FOUND）；删除成功返回 204（无响应体）。
//	@Tags			books
//	@Security		BearerAuth
//	@Param			book	path	string	true	"书籍资源 ID（ULID）"
//	@Success		204
//	@Failure		401	{object}	types.Problem
//	@Failure		404	{object}	types.Problem
//	@Router			/api/v1/books/{book} [delete]
func (h *BookHandler) Delete(c *echo.Context) error {
	var path types.BookPath
	if err := c.Bind(&path); err != nil {
		return err
	}

	if err := h.book.Delete(c.Request().Context(), path.Book); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}
