package httpkit

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

	"github.com/liukeshao/echo-template/pkg/httperr"
)

const (
	ContentTypeMergePatch = "application/merge-patch+json"
	ContentTypeJSON       = "application/json"
)

var SupportedPatchMediaTypes = []string{
	ContentTypeMergePatch,
	ContentTypeJSON,
}

// ApplyMergePatch 按 RFC 7386 JSON Merge Patch 将 body 合并到 original。
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
// 反序列化失败包装为 400 Problem 并附带 PatchInfo，不泄漏内部 Go 类型名。
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
		info := httperr.PatchInfo{
			ContentType: effectiveType,
			Reason:      httperr.PatchSchemaMismatch,
		}
		if typeErr, ok := errors.AsType[*json.UnmarshalTypeError](err); ok && typeErr.Field != "" {
			info.Field = typeErr.Field
		}
		return httperr.Problem{
			Status:   http.StatusBadRequest,
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
		return nil, invalidPatch(effectiveMediaType, httperr.PatchInvalidOriginal)
	}
	patchVal, err := decodeJSONValue(body)
	if err != nil {
		return nil, invalidPatch(effectiveMediaType, httperr.PatchInvalidJSON)
	}

	merged := mergeValue(target, patchVal)
	out, err := json.Marshal(merged)
	if err != nil {
		return nil, invalidPatch(effectiveMediaType, httperr.PatchMarshalFailed)
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
	return httperr.Problem{
		Status: http.StatusBadRequest,
		Metadata: httperr.PatchInfo{
			ContentType: mediaType,
			Reason:      reason,
		},
	}
}

func unsupportedPatchMediaType(actual string) error {
	return httperr.Problem{
		Status: http.StatusUnsupportedMediaType,
		Metadata: httperr.PatchInfo{
			ContentType: actual,
			Supported:   slices.Clone(SupportedPatchMediaTypes),
		},
	}
}
