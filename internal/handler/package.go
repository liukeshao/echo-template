package handler

import "github.com/samber/do/v2"

var Package = do.Package(
	do.Lazy(NewAuth),
	do.Lazy(NewBook),
	do.Lazy(NewMe),
)

// mapSlice 保证空切片返回非 nil（JSON 序列化为 [] 而非 null）。
func mapSlice[In, Out any](in []In, fn func(In) Out) []Out {
	out := make([]Out, len(in))
	for i, v := range in {
		out[i] = fn(v)
	}
	return out
}
