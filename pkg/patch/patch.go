// Package patch 实现 Kubernetes 风格的多协议 PATCH 操作。
//
// 根据请求的 Content-Type 选择不同的 patch 算法：
//
//   - application/json-patch+json         → RFC 6902 JSON Patch
//   - application/merge-patch+json        → RFC 7386 JSON Merge Patch
//   - application/merge-patch+shorthand   → Shorthand 语法 → JSON Merge Patch
//   - application/shorthand-patch+json    → 等同于 merge-patch+shorthand（兼容写法）
//   - application/json（或缺省）          → 兼容旧式，按 JSON Merge Patch 处理
//
// 典型调用流程（handler 层）：
//
//  1. service.Get(ctx, id) 拿到当前资源
//  2. 将当前资源序列化为 JSON（通常是对外的 Response DTO）
//  3. patch.Apply(current, body, contentType) 得到 patched JSON
//  4. json.Unmarshal(patched, &ReplaceRequest{})
//  5. ReplaceRequest.Validate()
//  6. service.Replace(ctx, id, &req) 写回，完成 read-modify-write
//
// 这种"GET → patch → PUT"流程与 Kubernetes apimachinery 的 PATCH 实现一致，
// 业务层只需暴露一个幂等的 Replace 接口即可天然支持三种 patch 协议。
package patch

import (
	"encoding/json"
	"fmt"
	"mime"
	"strings"

	"github.com/danielgtaylor/shorthand/v2"
	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/samber/oops"

	"github.com/liukeshao/echo-template/pkg/codes"
)

// Content type 常量。命名遵循 IANA 与 Kubernetes 既有惯例。
const (
	ContentTypeJSONPatch      = "application/json-patch+json"       // RFC 6902
	ContentTypeMergePatch     = "application/merge-patch+json"      // RFC 7386
	ContentTypeShorthandPatch = "application/merge-patch+shorthand" // Shorthand → merge patch
	ContentTypeShorthandAlt   = "application/shorthand-patch+json"  // 等价兼容形式
	ContentTypeJSON           = "application/json"                  // 兼容旧式 PATCH，按 merge patch 处理
)

// SupportedMediaTypes 列出 Apply 已支持的 Content-Type，便于在 Accept-Patch 响应头中回显。
var SupportedMediaTypes = []string{
	ContentTypeJSONPatch,
	ContentTypeMergePatch,
	ContentTypeShorthandPatch,
}

// Apply 根据 contentType 选择对应的 patch 算法，将 body 应用于 original。
//
// 参数：
//   - original    : 当前资源的 JSON 序列化结果
//   - body        : 客户端提交的 patch 主体
//   - contentType : HTTP Content-Type 头（允许带参数，例如 "...; charset=utf-8"）
//
// 返回：
//   - patched     : 应用 patch 后的 JSON 文档
//   - err         : 内部使用 oops 包装，包含 codes.PatchInvalid 或 codes.PatchUnsupportedMediaType
func Apply(original, body []byte, contentType string) ([]byte, error) {
	mediaType := parseMediaType(contentType)

	switch mediaType {
	case ContentTypeJSONPatch:
		return applyJSONPatch(original, body)
	case ContentTypeMergePatch, ContentTypeJSON, "":
		return applyMergePatch(original, body)
	case ContentTypeShorthandPatch, ContentTypeShorthandAlt:
		return applyShorthandPatch(original, body)
	default:
		return nil, unsupportedMediaType(contentType)
	}
}

// parseMediaType 提取 Content-Type 的主类型部分（去掉 charset、boundary 等参数），并小写化。
// 解析失败时回退到原始字符串的 trim+lower 形式，避免 Content-Type 不规范时直接 500。
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

// applyJSONPatch 执行 RFC 6902 JSON Patch。
func applyJSONPatch(original, body []byte) ([]byte, error) {
	p, err := jsonpatch.DecodePatch(body)
	if err != nil {
		return nil, invalidPatch(ContentTypeJSONPatch, fmt.Sprintf("invalid JSON Patch document: %v", err), err)
	}
	out, err := p.Apply(original)
	if err != nil {
		return nil, invalidPatch(ContentTypeJSONPatch, fmt.Sprintf("apply JSON Patch failed: %v", err), err)
	}
	return out, nil
}

// applyMergePatch 执行 RFC 7386 JSON Merge Patch。
// nil 字段表示删除（清空为 zero value 或可空字段的 null）。
func applyMergePatch(original, body []byte) ([]byte, error) {
	out, err := jsonpatch.MergePatch(original, body)
	if err != nil {
		return nil, invalidPatch(ContentTypeMergePatch, fmt.Sprintf("apply Merge Patch failed: %v", err), err)
	}
	return out, nil
}

// applyShorthandPatch 将 Shorthand DSL 叠加到 original 上。
//
// 实现策略与 Huma autopatch 一致：先将 original 解析为通用结构 tmp，
// 再把 tmp 作为 existing 传入 shorthand.Unmarshal —— Shorthand 解析器会在
// 现有结构上做 in-place merge。最终序列化回 JSON。
//
// 相对独立的"先解码 → 再 MergePatch"做法，这种方式：
//   - 支持 shorthand 中省略外层 {} 的紧凑写法（如 title: "New", price: 12.5）
//   - 自动遵循 shorthand 自身的删除/追加/路径语义
//
// Shorthand 文档：https://github.com/danielgtaylor/shorthand
func applyShorthandPatch(original, body []byte) ([]byte, error) {
	var tmp any
	if err := json.Unmarshal(original, &tmp); err != nil {
		return nil, invalidPatch(ContentTypeShorthandPatch, fmt.Sprintf("decode original document failed: %v", err), err)
	}
	tmp, shErr := shorthand.Unmarshal(string(body), shorthand.ParseOptions{
		ForceStringKeys:       true,
		EnableObjectDetection: true,
	}, tmp)
	if shErr != nil {
		return nil, invalidPatch(ContentTypeShorthandPatch, fmt.Sprintf("invalid Shorthand patch: %v", shErr), shErr)
	}
	out, err := json.Marshal(tmp)
	if err != nil {
		return nil, invalidPatch(ContentTypeShorthandPatch, fmt.Sprintf("marshal Shorthand result failed: %v", err), err)
	}
	return out, nil
}

// invalidPatch 构造 PATCH_INVALID oops 错误。
// public 消息携带具体的 content-type 与错误原因，便于客户端定位。
func invalidPatch(mediaType, msg string, cause error) error {
	return oops.
		Code(codes.PatchInvalid.String()).
		Tags("patch").
		With(codes.CtxKeyMetadata, map[string]string{
			"content_type": mediaType,
		}).
		Public(msg).
		Wrapf(cause, "patch invalid: %s", mediaType)
}

// unsupportedMediaType 构造 PATCH_UNSUPPORTED_MEDIA_TYPE oops 错误。
func unsupportedMediaType(actual string) error {
	supported := strings.Join(SupportedMediaTypes, ", ")
	return oops.
		Code(codes.PatchUnsupportedMediaType.String()).
		Tags("patch").
		With(codes.CtxKeyMetadata, map[string]string{
			"content_type": actual,
			"supported":    supported,
		}).
		Public(fmt.Sprintf("unsupported PATCH content type %q; supported: %s", actual, supported)).
		Errorf("unsupported patch media type: %s", actual)
}
