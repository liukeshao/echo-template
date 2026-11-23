package types

import "github.com/liukeshao/echo-template/pkg/validator"

// ListRequest 是所有标准集合列表接口的通用查询参数。
//
// 分页：page（从 1 开始，默认 1）+ page_size（默认 50，最大 1000）。
// 排序：order_by 逗号分隔，每段格式为 "field" 或 "field desc"。
// 过滤：filter 使用 AIP-160 表达式子集。
//
// 无额外查询条件的资源可直接绑定本类型；有扩展参数时可定义专属请求结构体。
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
