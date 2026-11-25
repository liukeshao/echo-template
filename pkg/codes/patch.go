package codes

import "net/http"

// Patch 域错误码，供 pkg/rest 在 RFC 7386 JSON Merge Patch 流程中使用：
//
//   - PatchInvalid              → 400 Bad Request（补丁文档语法或结构非法）
//   - PatchUnsupportedMediaType → 415 Unsupported Media Type（Content-Type 不在白名单内）
const (
	PatchInvalid              Code = "PATCH_INVALID"
	PatchUnsupportedMediaType Code = "PATCH_UNSUPPORTED_MEDIA_TYPE"
)

func init() {
	Register(PatchInvalid, http.StatusBadRequest)
	Register(PatchUnsupportedMediaType, http.StatusUnsupportedMediaType)
}
