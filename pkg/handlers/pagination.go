package handlers

import (
	metav1 "github.com/liukeshao/echo-template/pkg/meta/v1"
	"github.com/liukeshao/echo-template/pkg/types"
)

// defaultPageSize 和 maxPageSize 是分页的全局默认值。
// 若某资源需要不同的分页上限，在对应 handler 文件中单独声明常量并覆盖即可。
const (
	defaultPageSize = int32(50)
	maxPageSize     = int32(1000)
)

// resolvePagination 将 HTTP 请求中的 rawPage / rawSize 解析为业务层所需的 offset 和 limit。
//
//   - rawPage <= 0 时视为第 1 页。
//   - rawSize == 0 时使用 defaultSize；rawSize > maxSize 时截断为 maxSize。
//   - 返回的 page 是经过规范化后的实际页码（从 1 开始）。
func resolvePagination(rawPage, rawSize int32, defaultSize, maxSize int32) (offset, limit, page int32) {
	page = rawPage
	if page <= 0 {
		page = 1
	}

	limit = rawSize
	switch {
	case limit <= 0:
		limit = defaultSize
	case limit > maxSize:
		limit = maxSize
	}

	offset = (page - 1) * limit
	return offset, limit, page
}

// resolveListOptions 将通用列表请求解析为业务层 metav1.ListOptions 以及规范化后的页码。
func resolveListOptions(req types.ListRequest, defaultSize, maxSize int32) (metav1.ListOptions, int32) {
	offset, limit, page := resolvePagination(req.Page, req.PageSize, defaultSize, maxSize)
	return metav1.ListOptions{
		Offset:  offset,
		Limit:   limit,
		OrderBy: req.OrderBy,
		Filter:  req.Filter,
	}, page
}
