package v1

// ListOptions 是所有列表接口的通用查询选项。
type ListOptions struct {
	Offset  int32
	Limit   int32
	OrderBy string
	Filter  string
}
