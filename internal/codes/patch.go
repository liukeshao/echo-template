package codes

import (
	"net/http"

	"github.com/liukeshao/echo-template/pkg/rest"
)

// Patch 域错误码，与 pkg/rest 保持单一事实来源（SSOT）：
//
//   - PatchInvalid              → 400 Bad Request（补丁文档语法或结构非法）
//   - PatchUnsupportedMediaType → 415 Unsupported Media Type（Content-Type 不在白名单内）
const (
	PatchInvalid              Code = rest.CodePatchInvalid
	PatchUnsupportedMediaType Code = rest.CodePatchUnsupportedMediaType
)

func init() {
	Register(PatchInvalid, http.StatusBadRequest)
	Register(PatchUnsupportedMediaType, http.StatusUnsupportedMediaType)
}
