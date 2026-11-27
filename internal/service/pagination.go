package service

import "math"

const (
	defaultPageSize = int32(50)
	maxPageSize     = int32(1000)
)

type Page[T any] struct {
	Items     []T
	Page      int32
	PageSize  int32
	TotalSize int32
}

// resolvePagination 规范化页码与页大小，并以 int64 计算 offset 防止溢出。
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
