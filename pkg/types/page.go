package types

import (
	"math"

	z "github.com/Oudwins/zog"

	"github.com/liukeshao/echo-template/pkg/apperrs"
)

// 分页常量
const (
	DefaultPage     = 1   // 默认页码
	DefaultPageSize = 20  // 默认每页数量
	MaxPageSize     = 100 // 最大每页数量
)

type PageInput struct {
	Page     int `query:"page"`      // 页码
	PageSize int `query:"page_size"` // 每页数量
}

// Validate 验证分页输入
func (i *PageInput) Validate() *apperrs.Response {
	issuesMap := z.Struct(i.Shape()).Validate(i)
	if issuesMap != nil {
		return &apperrs.Response{
			Code:   400,
			Errors: FormatIssuesAsErrorDetails(issuesMap),
		}
	}
	return nil
}

func (i PageInput) Shape() z.Shape {
	return z.Shape{
		"Page":     z.Int().GTE(1).Default(DefaultPage),
		"PageSize": z.Int().GTE(1).LTE(MaxPageSize).Default(DefaultPageSize),
	}
}

// Offset 计算数据库查询偏移量
func (i PageInput) Offset() int {
	return (i.Page - 1) * i.PageSize
}

// Limit 返回查询限制数量（与PageSize相同，提供语义化方法）
func (i PageInput) Limit() int {
	return i.PageSize
}

type PageOutput struct {
	Page      int  `json:"page"`       // 页码
	PageSize  int  `json:"page_size"`  // 每页数量
	Total     int  `json:"total"`      // 总数
	TotalPage int  `json:"total_page"` // 总页数
	HasNext   bool `json:"has_next"`   // 是否有下一页
	HasPrev   bool `json:"has_prev"`   // 是否有上一页
}

// NewPageOutput 创建分页输出
func NewPageOutput(input PageInput, total int) PageOutput {
	totalPage := int(math.Ceil(float64(total) / float64(input.PageSize)))

	return PageOutput{
		Page:      input.Page,
		PageSize:  input.PageSize,
		Total:     total,
		TotalPage: totalPage,
		HasNext:   input.Page < totalPage,
		HasPrev:   input.Page > 1,
	}
}

// IsEmpty 检查是否为空页面
func (p PageOutput) IsEmpty() bool {
	return p.Total == 0
}

// IsValidPage 检查当前页是否有效
func (p PageOutput) IsValidPage() bool {
	return p.Page > 0 && p.Page <= p.TotalPage
}

// ValidatePageRequest 验证分页请求并返回错误响应
func ValidatePageRequest(page PageInput) *apperrs.Response {
	return page.Validate()
}

// NormalizePage 标准化分页参数，设置默认值
func NormalizePage(page *PageInput) {
	if page.Page <= 0 {
		page.Page = DefaultPage
	}
	if page.PageSize <= 0 {
		page.PageSize = DefaultPageSize
	}
	if page.PageSize > MaxPageSize {
		page.PageSize = MaxPageSize
	}
}
