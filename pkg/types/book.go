package types

import (
	"strings"
	"time"

	"github.com/liukeshao/echo-template/pkg/utils"
	"github.com/liukeshao/echo-template/pkg/validator"
)

// Book 是书籍资源的对外 JSON 表示。
// handler 层将 *ent.Book 映射到此类型后返回，隐藏 deleted_at 等内部字段。
//
// 同时作为 PATCH "read-modify-write" 的中间 JSON 表示：
// PATCH handler 会先 GET 资源、序列化为该结构、应用 patch、再反序列化为 ReplaceBookRequest 写回。
type Book struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Author      string     `json:"author"`
	Description string     `json:"description,omitempty"`
	Price       *float64   `json:"price,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// BookList 是书籍集合的分页列表表示。
type BookList struct {
	Items     []Book `json:"items"`
	Page      int32  `json:"page"`
	PageSize  int32  `json:"page_size"`
	TotalSize int32  `json:"total_size"`
}

// BookPath 绑定单本书籍路由的路径参数 {book}（书籍 ULID），
// 供 GET / DELETE / PATCH 等仅需路径参数的 handler 复用。
type BookPath struct {
	Book string `param:"book"`
}

// CreateBookRequest 创建书籍请求。
//
// book_id 为可选字段，若提供则用作资源的最终 ID（必须为合法 ULID 格式）；
// 若省略则由服务端自动生成。若指定的 book_id 已存在，服务返回 409 BOOK_ALREADY_EXISTS。
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
	validateBookFields(&v, r.Title, r.Author, r.Description, r.Price)
	return v.Err()
}

// ReplaceBookRequest 替换（PUT）书籍请求 —— 幂等全量更新。
//
// 语义：请求体描述资源期望的完整状态，所有可选字段若未提供将被清除。
// PATCH handler 在应用 RFC 6902 / 7386 / Shorthand patch 后，
// 会将合并结果反序列化到本结构再调用 service.Replace。
//
// id / created_at / updated_at 为只读字段，不出现在本结构中，
// 即便客户端在 PATCH 中尝试修改也会在反序列化阶段自动丢弃。
type ReplaceBookRequest struct {
	// Book 对应路径参数 {book}，即书籍 ULID。
	// PATCH handler 在应用补丁后会显式覆写该字段；PUT handler 通过 c.Bind 注入。
	Book        string     `param:"book" json:"-"`
	Title       string     `json:"title"`
	Author      string     `json:"author"`
	Description string     `json:"description,omitempty"`
	Price       *float64   `json:"price,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

func (r *ReplaceBookRequest) Validate() error {
	var v validator.FieldViolations
	validateBookFields(&v, r.Title, r.Author, r.Description, r.Price)
	return v.Err()
}

func validateBookFields(v *validator.FieldViolations, title, author, description string, price *float64) {
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
