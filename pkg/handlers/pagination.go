package handlers

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
