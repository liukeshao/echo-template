package types

import (
	"strings"
	"time"

	"github.com/liukeshao/echo-template/pkg/utils"
	"github.com/liukeshao/echo-template/pkg/validator"
)

// validBookUpdateFieldsList 与 validBookUpdateFields 同步，供错误消息模板复用。
const validBookUpdateFieldsList = "title, author, description, price, published_at"

// BookResponse 书籍响应 DTO。
// handler 层将 *ent.Book 映射到此类型后返回，
// 确保响应结构与存储层解耦，不会泄漏 deleted_at 等内部字段。
type BookResponse struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Author      string     `json:"author"`
	Description string     `json:"description,omitempty"`
	Price       float64    `json:"price,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CreateBookRequest 创建书籍请求（AIP-133）。
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
	Price       float64    `json:"price,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

func (r *CreateBookRequest) Validate() error {
	var v validator.FieldViolations
	if r.BookID != "" && !utils.IsValidULID(r.BookID) {
		v.Add("book_id", validator.KeyULID, map[string]string{"field": "book_id"})
	}
	if strings.TrimSpace(r.Title) == "" {
		v.Add("title", validator.KeyRequired, map[string]string{"field": "title"})
	} else if len(r.Title) > 200 {
		v.Add("title", validator.KeyMaxLength, map[string]string{"field": "title", "max": "200"})
	}
	if strings.TrimSpace(r.Author) == "" {
		v.Add("author", validator.KeyRequired, map[string]string{"field": "author"})
	} else if len(r.Author) > 100 {
		v.Add("author", validator.KeyMaxLength, map[string]string{"field": "author", "max": "100"})
	}
	if r.Price < 0 {
		v.Add("price", validator.KeyNonNegative, map[string]string{"field": "price"})
	}
	return v.Err()
}

// GetBookRequest 获取单本书籍的请求。
// Book 对应路径参数 {book}，即书籍的 ULID。
type GetBookRequest struct {
	Book string `param:"book"`
}

// DeleteBookRequest 删除书籍请求（AIP-135）。
// Book 对应路径参数 {book}，即书籍的 ULID。
// 响应为空（HTTP 204）；资源不存在时返回 NOT_FOUND。
type DeleteBookRequest struct {
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

// validBookUpdateFields 是 update_mask 中合法的字段名集合（对应资源字段，非请求字段）。
var validBookUpdateFields = map[string]struct{}{
	"title": {}, "author": {}, "description": {}, "price": {}, "published_at": {},
}

// UpdateBookRequest 更新书籍请求（AIP-134）。
//
// update_mask 控制哪些字段被更新：
//   - 省略（空）→ 更新请求体中所有已设置的字段（非 nil）
//   - ["*"] → 全量替换；title/author 必填，可选字段若未提供则被清除
//   - ["title","price",...] → 仅更新 mask 中列出的字段；
//     可选字段在 mask 中但请求体中为 nil 时，将被清除
type UpdateBookRequest struct {
	// Book 对应路径参数 {book}，即书籍 ULID。
	Book string `param:"book"`
	// UpdateMask 字段掩码（可选）。有效字段名：title, author, description, price, published_at。
	// 特殊值 * 表示全量替换。
	UpdateMask  []string   `json:"update_mask,omitempty"`
	Title       *string    `json:"title,omitempty"`
	Author      *string    `json:"author,omitempty"`
	Description *string    `json:"description,omitempty"`
	Price       *float64   `json:"price,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

func (r *UpdateBookRequest) Validate() error {
	var v validator.FieldViolations

	// 校验 update_mask 中的字段名是否合法
	hasWildcard := false
	for _, f := range r.UpdateMask {
		if f == "*" {
			hasWildcard = true
			continue
		}
		if _, ok := validBookUpdateFields[f]; !ok {
			v.Add("update_mask", validator.KeyUnknownField, map[string]string{
				"field":   f,
				"allowed": validBookUpdateFieldsList,
			})
		}
	}
	if err := v.Err(); err != nil {
		return err
	}

	// mask 省略时，至少需要提供一个字段
	if len(r.UpdateMask) == 0 {
		if r.Title == nil && r.Author == nil && r.Description == nil && r.Price == nil && r.PublishedAt == nil {
			v.Add("", validator.KeyAtLeastOne, nil)
			return v.Err()
		}
	}

	// mask=* 全量替换时，必填字段不能缺失
	if hasWildcard {
		if r.Title == nil {
			v.Add("title", validator.KeyRequiredForReplace, map[string]string{"field": "title"})
		}
		if r.Author == nil {
			v.Add("author", validator.KeyRequiredForReplace, map[string]string{"field": "author"})
		}
	}

	// 校验请求体中已提供的字段值
	if r.Title != nil {
		if strings.TrimSpace(*r.Title) == "" {
			v.Add("title", validator.KeyNotBlank, map[string]string{"field": "title"})
		} else if len(*r.Title) > 200 {
			v.Add("title", validator.KeyMaxLength, map[string]string{"field": "title", "max": "200"})
		}
	}
	if r.Author != nil {
		if strings.TrimSpace(*r.Author) == "" {
			v.Add("author", validator.KeyNotBlank, map[string]string{"field": "author"})
		} else if len(*r.Author) > 100 {
			v.Add("author", validator.KeyMaxLength, map[string]string{"field": "author", "max": "100"})
		}
	}
	if r.Price != nil && *r.Price < 0 {
		v.Add("price", validator.KeyNonNegative, map[string]string{"field": "price"})
	}
	return v.Err()
}
