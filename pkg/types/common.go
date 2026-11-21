package types

import (
	"time"

	"github.com/liukeshao/echo-template/pkg/validator"
)

// ObjectMeta 包含所有标准资源共享的只读系统字段（对应 ent/schema.DefaultMixin 的对外投影）。
//
// 仅嵌入在资源响应 DTO 中，不出现在 Create / Replace 请求中，
// 因此 PUT 与 PATCH 反序列化到 ReplaceXxxRequest 时会自动丢弃只读字段。
type ObjectMeta struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListRequest 是所有标准集合列表接口的通用查询参数。
//
// 分页：page（从 1 开始，默认 1）+ page_size（默认 50，最大 1000）。
// 排序：order_by 逗号分隔，每段格式为 "field" 或 "field desc"。
// 过滤：filter 使用 AIP-160 表达式子集。
//
// 无额外查询条件的资源可直接绑定本类型；有扩展参数时可匿名嵌入并复用 Validate()。
type ListRequest struct {
	Page     int32  `query:"page"`
	PageSize int32  `query:"page_size"`
	OrderBy  string `query:"order_by"`
	Filter   string `query:"filter"`
}

// Validate 校验分页参数不为负数。
func (r *ListRequest) Validate() error {
	var v validator.FieldViolations
	if r.Page < 0 {
		v.Add("page", validator.KeyNonNegative, map[string]string{"field": "page"})
	}
	if r.PageSize < 0 {
		v.Add("page_size", validator.KeyNonNegative, map[string]string{"field": "page_size"})
	}
	return v.Err()
}

// PageMeta 描述分页列表响应的元信息。
type PageMeta struct {
	Page      int32 `json:"page"`
	PageSize  int32 `json:"page_size"`
	TotalSize int32 `json:"total_size"`
}

// ListResponse 是所有集合列表接口的通用泛型响应（Kubernetes 风格 items + 分页元信息）。
type ListResponse[T any] struct {
	Items []T `json:"items"`
	PageMeta
}

// NewListResponse 构造 ListResponse，并保证 items 永不为 nil（空列表序列化为 [] 而非 null）。
func NewListResponse[T any](items []T, page, pageSize, totalSize int32) ListResponse[T] {
	if items == nil {
		items = []T{}
	}
	return ListResponse[T]{
		Items: items,
		PageMeta: PageMeta{
			Page:      page,
			PageSize:  pageSize,
			TotalSize: totalSize,
		},
	}
}
