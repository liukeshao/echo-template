package handlers

import (
	"github.com/labstack/echo/v5"
	"github.com/samber/do/v2"

	"github.com/liukeshao/echo-template/ent"
	metav1 "github.com/liukeshao/echo-template/pkg/meta/v1"
	"github.com/liukeshao/echo-template/pkg/middleware"
	"github.com/liukeshao/echo-template/pkg/services"
	"github.com/liukeshao/echo-template/pkg/types"
)

// BookHandler 书籍处理器
type BookHandler struct {
	book   *services.BookService
	authMW middleware.AuthMiddleware
}

// NewBookHandler 构造 BookHandler 并注入依赖。
func NewBookHandler(i do.Injector) (*BookHandler, error) {
	return &BookHandler{
		book:   do.MustInvoke[*services.BookService](i),
		authMW: do.MustInvoke[middleware.AuthMiddleware](i),
	}, nil
}

// Routes 注册路由
func (h *BookHandler) Routes(g *echo.Group) {
	protected := g.Group("/api/v1/books")
	protected.Use(echo.MiddlewareFunc(h.authMW))

	protected.POST("", h.Create)
	protected.GET("", h.List)
	protected.GET("/:book", h.Get)
	protected.PATCH("/:book", h.Update)
	protected.DELETE("/:book", h.Delete)
}

// toBookResponse maps *ent.Book to types.BookResponse, hiding internal fields such as deleted_at.
func toBookResponse(b *ent.Book) types.BookResponse {
	return types.BookResponse{
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

// toBookResponses maps a slice of *ent.Book to []types.BookResponse.
func toBookResponses(books []*ent.Book) []types.BookResponse {
	out := make([]types.BookResponse, len(books))
	for i, b := range books {
		out[i] = toBookResponse(b)
	}
	return out
}

// Create 创建书籍
//
//	@Summary		创建书籍
//	@Description	AIP-133：book_id 可选，若指定则用作资源 ID（ULID 格式）；若省略则服务端自动生成。book_id 重复时返回 409。
//	@Tags			books
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		types.CreateBookRequest	true	"书籍信息"
//	@Success		201		{object}	types.BookResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse	"book_id 已存在（ALREADY_EXISTS）"
//	@Router			/api/v1/books [post]
func (h *BookHandler) Create(c *echo.Context) error {
	var req types.CreateBookRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}

	b, err := h.book.Create(c.Request().Context(), &req)
	if err != nil {
		return err
	}

	return c.JSON(201, toBookResponse(b))
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
//	@Param		filter		query		string	false	"过滤表达式，如 'title : \"Go\" AND price < 50'"
//	@Success	200			{object}	types.ListBooksResponse
//	@Failure	400			{object}	ErrorResponse
//	@Failure	401			{object}	ErrorResponse
//	@Router		/api/v1/books [get]
func (h *BookHandler) List(c *echo.Context) error {
	var req types.ListBooksRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}

	offset, limit, page := resolvePagination(req.Page, req.PageSize, defaultPageSize, maxPageSize)

	books, total, err := h.book.List(c.Request().Context(), metav1.ListOptions{
		Offset:  offset,
		Limit:   limit,
		OrderBy: req.OrderBy,
		Filter:  req.Filter,
	})
	if err != nil {
		return err
	}

	return c.JSON(200, types.ListBooksResponse{
		Books:     toBookResponses(books),
		Page:      page,
		PageSize:  limit,
		TotalSize: total,
	})
}

// Get 获取书籍详情
//
//	@Summary	获取书籍详情
//	@Tags		books
//	@Security	BearerAuth
//	@Produce	json
//	@Param		book	path		string	true	"书籍资源名称（书籍 ULID）"
//	@Success	200		{object}	types.BookResponse
//	@Failure	401		{object}	ErrorResponse
//	@Failure	404		{object}	ErrorResponse
//	@Router		/api/v1/books/{book} [get]
func (h *BookHandler) Get(c *echo.Context) error {
	var req types.GetBookRequest
	if err := c.Bind(&req); err != nil {
		return err
	}

	b, err := h.book.Get(c.Request().Context(), req.Book)
	if err != nil {
		return err
	}

	return c.JSON(200, toBookResponse(b))
}

// Update 更新书籍
//
//	@Summary		更新书籍
//	@Description	AIP-134：支持 update_mask 字段掩码。省略时更新所有已提供字段；["*"] 表示全量替换；指定字段名则仅更新对应字段。
//	@Tags			books
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			book	path		string					true	"书籍资源名称（书籍 ULID）"
//	@Param			body	body		types.UpdateBookRequest	true	"更新信息"
//	@Success		200		{object}	types.BookResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Router			/api/v1/books/{book} [patch]
func (h *BookHandler) Update(c *echo.Context) error {
	var req types.UpdateBookRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}

	b, err := h.book.Update(c.Request().Context(), req.Book, &req)
	if err != nil {
		return err
	}

	return c.JSON(200, toBookResponse(b))
}

// Delete 删除书籍
//
//	@Summary		删除书籍
//	@Description	AIP-135：资源不存在时返回 404（NOT_FOUND）；删除成功返回 204（无响应体）。
//	@Tags			books
//	@Security		BearerAuth
//	@Param			book	path	string	true	"书籍资源名称（书籍 ULID）"
//	@Success		204
//	@Failure		401	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Router			/api/v1/books/{book} [delete]
func (h *BookHandler) Delete(c *echo.Context) error {
	var req types.DeleteBookRequest
	if err := c.Bind(&req); err != nil {
		return err
	}

	if err := h.book.Delete(c.Request().Context(), req.Book); err != nil {
		return err
	}

	return c.NoContent(204)
}
