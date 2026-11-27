package handler

import "github.com/samber/do/v2"

// Package 注册全部业务领域处理器到 DI 容器。
var Package = do.Package(
	do.Lazy(NewAuth),
	do.Lazy(NewBook),
	do.Lazy(NewMe),
)

// mapSlice 将源切片逐元素映射为目标切片，保证返回值永不为 nil（空切片 JSON 序列化为 []）。
func mapSlice[In, Out any](in []In, fn func(In) Out) []Out {
	out := make([]Out, len(in))
	for i, v := range in {
		out[i] = fn(v)
	}
	return out
}
