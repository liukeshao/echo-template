package services

import "math"

// defaultPageSize 和 maxPageSize 是分页的全局默认值。
// 若某资源需要不同的分页上限，在对应 service 文件中单独传入覆盖即可。
const (
	defaultPageSize = int32(50)
	maxPageSize     = int32(1000)
)

// Page 是列表查询的通用分页结果，包含当前页数据与规范化后的分页元信息。
type Page[T any] struct {
	Items     []T
	Page      int32
	PageSize  int32
	TotalSize int32
}

// resolvePagination 将请求中的 rawPage / rawSize 解析为数据库查询所需的 offset、limit 以及规范化后的页码。
//
//   - rawPage <= 0 时视为第 1 页。
//   - rawSize <= 0 时使用 defaultSize；rawSize > maxSize 时截断为 maxSize。
//   - offset 使用 int64 计算后再收窄到 int，避免大页码乘以 page_size 时发生 int32 溢出变负。
func resolvePagination(rawPage, rawSize int32, defaultSize, maxSize int32) (offset int, limit, page int32) {
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

	off64 := int64(page-1) * int64(limit)
	if off64 > int64(math.MaxInt) {
		offset = math.MaxInt
	} else {
		offset = int(off64)
	}
	return offset, limit, page
}
