package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
)

// RFC 7386 JSON Merge Patch 支持的 Content-Type 常量。
const (
	ContentTypeMergePatch = "application/merge-patch+json" // RFC 7386
	ContentTypeJSON       = "application/json"             // 兼容写法，按 Merge Patch 处理
)

// SupportedPatchMediaTypes 列出 ApplyMergePatch 支持的 Content-Type，用于 415 提示与 Accept-Patch 回显。
var SupportedPatchMediaTypes = []string{
	ContentTypeMergePatch,
	ContentTypeJSON,
}

// ApplyMergePatch 按 RFC 7386 JSON Merge Patch 将 body 合并到 original 上。
//
// 参数：
//   - original    : 当前资源的 JSON 序列化结果
//   - body        : 客户端提交的 JSON Merge Patch 主体（null 表示清空/删除字段）
//   - contentType : HTTP Content-Type 头（允许带参数，例如 "...; charset=utf-8"）
//
// 返回错误为 Problem，包含 CodePatchInvalid（400）或 CodePatchUnsupportedMediaType（415）及结构化 Params。
func ApplyMergePatch(original, body []byte, contentType string) ([]byte, error) {
	mediaType := parseMediaType(contentType)

	switch mediaType {
	case ContentTypeMergePatch, ContentTypeJSON, "":
		return applyMergePatch(original, body)
	default:
		return nil, unsupportedPatchMediaType(contentType)
	}
}

// ApplyMergePatchInto 执行 ApplyMergePatch 并将结果反序列化到 dst（通常为 *api.ReplaceXxxRequest）。
// 反序列化失败（例如字段类型不匹配）包装为 PATCH_INVALID（HTTP 400），避免漏出 500。
func ApplyMergePatchInto(original, body []byte, contentType string, dst any) error {
	patched, err := ApplyMergePatch(original, body, contentType)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(patched, dst); err != nil {
		return invalidPatch(fmt.Sprintf("patched document does not match target schema: %v", err))
	}
	return nil
}

func parseMediaType(ct string) string {
	if ct == "" {
		return ""
	}
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(ct))
	}
	return mt
}

// applyMergePatch 执行 RFC 7386 Section 2 定义的 JSON Merge Patch 算法。
func applyMergePatch(original, body []byte) ([]byte, error) {
	target, err := decodeJSONValue(original)
	if err != nil {
		return nil, invalidPatch(fmt.Sprintf("decode original document failed: %v", err))
	}
	patchVal, err := decodeJSONValue(body)
	if err != nil {
		return nil, invalidPatch(fmt.Sprintf("invalid JSON Merge Patch document: %v", err))
	}

	merged := mergeValue(target, patchVal)
	out, err := json.Marshal(merged)
	if err != nil {
		return nil, invalidPatch(fmt.Sprintf("marshal merged document failed: %v", err))
	}
	return out, nil
}

func decodeJSONValue(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("unexpected trailing data after JSON value")
	}
	return v, nil
}

func mergeValue(target, patchVal any) any {
	patchObj, ok := patchVal.(map[string]any)
	if !ok {
		return patchVal
	}
	targetObj, ok := target.(map[string]any)
	if !ok {
		targetObj = make(map[string]any, len(patchObj))
	}
	for k, v := range patchObj {
		if v == nil {
			delete(targetObj, k)
		} else {
			targetObj[k] = mergeValue(targetObj[k], v)
		}
	}
	return targetObj
}

func invalidPatch(msg string) error {
	return Problem{
		Status: http.StatusBadRequest,
		Code:   CodePatchInvalid,
		Detail: msg,
		Params: map[string]string{
			"content_type": ContentTypeMergePatch,
		},
	}
}

func unsupportedPatchMediaType(actual string) error {
	supported := strings.Join(SupportedPatchMediaTypes, ", ")
	return Problem{
		Status: http.StatusUnsupportedMediaType,
		Code:   CodePatchUnsupportedMediaType,
		Detail: fmt.Sprintf("unsupported PATCH content type %q; supported: %s", actual, supported),
		Params: map[string]string{
			"content_type": actual,
			"supported":    supported,
		},
	}
}
