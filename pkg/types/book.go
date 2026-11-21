package types

import (
	"strings"
	"time"

	"github.com/liukeshao/echo-template/pkg/utils"
	"github.com/liukeshao/echo-template/pkg/validator"
)

// BookResponse 书籍响应 DTO。
// handler 层将 *ent.Book 映射到此类型后返回，
// 确保响应结构与存储层解耦，不会泄漏 deleted_at 等内部字段。
//
// 该结构同时作为 PATCH "read-modify-write" 的中间 JSON 表示：
// PATCH handler 会先 GET 资源、序列化为该结构、应用 patch、再反序列化为 ReplaceBookRequest 写回。
type BookResponse struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Author      string     `json:"author"`
	Description string     `json:"description,omitempty"`
	Price       *float64   `json:"price,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CreateBookRequest 创建书籍请求。
//
// book_id 为可选字段，若提供则用作资源的最终 ID（必须为合法 ULID 格式）；
// 若省略则由服务端自动生成。若指定的 book_id 已存在，服务返回 ALREADY_EXISTS 错误。
type CreateBookRequest struct {
	// BookID 用户自定义 ID（可选）。格式：26 位 ULID（大写字母 + 数字）。
	// 若省略，服务端自动生成。
	BookID      string     `json:"book_id,omitempty"`
	Title       string     `json:"title"`
	Author      string     `json:"author"`
	Description string     `json:"description,omitempty"`
	Price       *float64   `json:"price,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

func (r *CreateBookRequest) Validate() error {
	var v validator.FieldViolations
	if r.BookID != "" && !utils.IsValidULID(r.BookID) {
		v.Add("book_id", validator.KeyULID, map[string]string{"field": "book_id"})
	}
	validateBookWritableFields(&v, r.Title, r.Author, r.Description, r.Price)
	return v.Err()
}

// GetBookRequest 获取单本书籍的请求。
// Book 对应路径参数 {book}，即书籍的 ULID。
type GetBookRequest struct {
	Book string `param:"book"`
}

// DeleteBookRequest 删除书籍请求。
// Book 对应路径参数 {book}，即书籍的 ULID。
// 响应为空（HTTP 204）；资源不存在时返回 NOT_FOUND。
type DeleteBookRequest struct {
	Book string `param:"book"`
}

// PatchBookRequest 仅用于 PATCH handler 在不解析 body 的情况下取出路径参数。
//
// 之所以单独定义而不复用 GetBookRequest：
//   - 语义上 PATCH 与 GET 不同；类型独立利于后续按需扩展（例如 strategic-merge 标签开关）
//   - PATCH 的 body 形态由 Content-Type 决定（可能是 JSON Patch 数组，无法套到任何 struct），
//     因此 handler 需要使用 echo.BindPathValues 而非全量 c.Bind，保留 body 给 patch.Apply 独占处理。
type PatchBookRequest struct {
	Book string `param:"book"`
}

// ListBooksRequest 列表请求。
//
// 分页：page（从 1 开始，默认 1）+ page_size（默认 50，最大 1000）。
// 排序：order_by 逗号分隔，每段格式为 "field" 或 "field desc"。
//
//	支持字段：title, author, price, created_at, updated_at, published_at。
//	默认：created_at desc。
//
// 过滤：filter 使用 AIP-160 子集表达式，多条件用 AND 连接。
//
//	字符串字段（title, author）：= "value"、!= "value"、: "value"（含，不区分大小写）。
//	数值字段（price）：=、!=、<、<=、>、>= 后跟数字。
//	示例：title : "Go" AND price < 50
type ListBooksRequest struct {
	Page     int32  `query:"page"`
	PageSize int32  `query:"page_size"`
	OrderBy  string `query:"order_by"`
	Filter   string `query:"filter"`
}

func (r *ListBooksRequest) Validate() error {
	var v validator.FieldViolations
	if r.Page < 0 {
		v.Add("page", validator.KeyNonNegative, map[string]string{"field": "page"})
	}
	if r.PageSize < 0 {
		v.Add("page_size", validator.KeyNonNegative, map[string]string{"field": "page_size"})
	}
	return v.Err()
}

// ListBooksResponse 列表响应。
type ListBooksResponse struct {
	Books     []BookResponse `json:"books"`
	Page      int32          `json:"page"`
	PageSize  int32          `json:"page_size"`
	TotalSize int32          `json:"total_size"`
}

// ReplaceBookRequest 替换（PUT）书籍请求 —— Kubernetes 风格的幂等全量更新。
//
// 语义：请求体描述资源期望的完整状态，所有可选字段若未提供将被清除。
// PATCH handler 在应用 RFC 6902 / 7386 / Shorthand patch 后，
// 会将合并结果反序列化到本结构再调用 service.Replace。
//
// 该结构故意不包含 id / created_at / updated_at —— 这些是只读字段，
// 即便客户端在 PATCH 中尝试修改也会被这里的 JSON 反序列化直接忽略。
type ReplaceBookRequest struct {
	// Book 对应路径参数 {book}，即书籍 ULID。
	// PATCH handler 在应用补丁后会显式覆写该字段；PUT handler 通过 echo.Bind 注入。
	Book string `param:"book" json:"-"`

	Title       string     `json:"title"`
	Author      string     `json:"author"`
	Description string     `json:"description,omitempty"`
	Price       *float64   `json:"price,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

func (r *ReplaceBookRequest) Validate() error {
	var v validator.FieldViolations
	validateBookWritableFields(&v, r.Title, r.Author, r.Description, r.Price)
	return v.Err()
}

// validateBookWritableFields 集中处理 title / author / description / price 的字段级校验，
// 在 Create 与 Replace 两条路径上共享，避免规则发散。
func validateBookWritableFields(v *validator.FieldViolations, title, author, description string, price *float64) {
	if strings.TrimSpace(title) == "" {
		v.Add("title", validator.KeyRequired, map[string]string{"field": "title"})
	} else if len(title) > 200 {
		v.Add("title", validator.KeyMaxLength, map[string]string{"field": "title", "max": "200"})
	}
	if strings.TrimSpace(author) == "" {
		v.Add("author", validator.KeyRequired, map[string]string{"field": "author"})
	} else if len(author) > 100 {
		v.Add("author", validator.KeyMaxLength, map[string]string{"field": "author", "max": "100"})
	}
	if len(description) > 1000 {
		v.Add("description", validator.KeyMaxLength, map[string]string{"field": "description", "max": "1000"})
	}
	if price != nil && *price < 0 {
		v.Add("price", validator.KeyNonNegative, map[string]string{"field": "price"})
	}
}
