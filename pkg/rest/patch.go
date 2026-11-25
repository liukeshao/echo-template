package rest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"slices"
	"strings"
)

// RFC 7386 JSON Merge Patch 支持的 Content-Type 常量。
const (
	ContentTypeMergePatch = "application/merge-patch+json" // RFC 7386
	ContentTypeJSON       = "application/json"             // 兼容写法，按 Merge Patch 处理
)

// SupportedPatchMediaTypes 列出 ApplyMergePatch 支持的 Content-Type，用于 415 MediaTypeInfo 与 Accept-Patch 回显。
var SupportedPatchMediaTypes = []string{
	ContentTypeMergePatch,
	ContentTypeJSON,
}

// ApplyMergePatch 按 RFC 7386 JSON Merge Patch 将 body 合并到 original 上。
//
// 返回错误为 Problem（无硬编码 Detail），包含 CodePatchInvalid（400）或 CodePatchUnsupportedMediaType（415）及 MediaTypeInfo。
func ApplyMergePatch(original, body []byte, contentType string) ([]byte, error) {
	mediaType := parseMediaType(contentType)

	switch mediaType {
	case ContentTypeMergePatch, ContentTypeJSON, "":
		effectiveType := mediaType
		if effectiveType == "" {
			effectiveType = ContentTypeMergePatch
		}
		return applyMergePatch(original, body, effectiveType)
	default:
		return nil, unsupportedPatchMediaType(contentType)
	}
}

// ApplyMergePatchInto 执行 ApplyMergePatch 并将结果反序列化到 dst（通常为 *api.ReplaceXxxRequest）。
// 反序列化失败包装为 PATCH_INVALID（HTTP 400）并附带 MediaTypeInfo，不泄漏内部 Go 类型名。
func ApplyMergePatchInto(original, body []byte, contentType string, dst any) error {
	patched, err := ApplyMergePatch(original, body, contentType)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(patched, dst); err != nil {
		effectiveType := parseMediaType(contentType)
		if effectiveType == "" {
			effectiveType = ContentTypeMergePatch
		}
		info := MediaTypeInfo{
			ContentType: effectiveType,
			Reason:      "schema_mismatch",
		}
		if typeErr, ok := errors.AsType[*json.UnmarshalTypeError](err); ok && typeErr.Field != "" {
			info.Field = typeErr.Field
		}
		return Problem{
			Status:   http.StatusBadRequest,
			Code:     CodePatchInvalid,
			Metadata: info,
		}
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
func applyMergePatch(original, body []byte, effectiveMediaType string) ([]byte, error) {
	target, err := decodeJSONValue(original)
	if err != nil {
		return nil, invalidPatch(effectiveMediaType, "invalid_original")
	}
	patchVal, err := decodeJSONValue(body)
	if err != nil {
		return nil, invalidPatch(effectiveMediaType, "invalid_json")
	}

	merged := mergeValue(target, patchVal)
	out, err := json.Marshal(merged)
	if err != nil {
		return nil, invalidPatch(effectiveMediaType, "marshal_failed")
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

func invalidPatch(mediaType, reason string) error {
	return Problem{
		Status: http.StatusBadRequest,
		Code:   CodePatchInvalid,
		Metadata: MediaTypeInfo{
			ContentType: mediaType,
			Reason:      reason,
		},
	}
}

func unsupportedPatchMediaType(actual string) error {
	return Problem{
		Status: http.StatusUnsupportedMediaType,
		Code:   CodePatchUnsupportedMediaType,
		Metadata: MediaTypeInfo{
			ContentType: actual,
			Supported:   slices.Clone(SupportedPatchMediaTypes),
		},
	}
}
