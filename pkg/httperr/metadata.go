package httperr

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// MetaKey 是在 oops.With(httperr.MetaKey, ...) 中存放对外公开 AIP-193 Metadata Struct 的标准键名。
// 仅该键下实现 Metadata 接口的值会被提取到 Problem.Metadata 并供给最终层 i18n 插值；
// 其他 oops.With(...) 键仅用于服务端 slog 结构化日志，天然物理隔离。
const MetaKey = "metadata"

// DecodeInfo.Reason 常量。
const (
	DecodeReasonEmptyBody    = "empty_body"
	DecodeReasonInvalidJSON  = "invalid_json"
	DecodeReasonTrailingData = "trailing_data"
)

// MediaTypeInfo.Reason 常量。
const (
	PatchReasonInvalidOriginal = "invalid_original"
	PatchReasonInvalidJSON     = "invalid_json"
	PatchReasonMarshalFailed   = "marshal_failed"
	PatchReasonSchemaMismatch  = "schema_mismatch"
)

// PreconditionInfo.Type 通用常量（各业务领域可按 UPPER_SNAKE_CASE 自行扩展）。
const (
	PreconditionInvalidCredentials = "INVALID_CREDENTIALS"
	PreconditionMissingHeader      = "MISSING_HEADER"
	PreconditionMalformedHeader    = "MALFORMED_HEADER"
	PreconditionEmptyToken         = "EMPTY_TOKEN"
	PreconditionTokenExpired       = "TOKEN_EXPIRED"
	PreconditionTokenInvalid       = "TOKEN_INVALID"
	PreconditionTokenRevoked       = "TOKEN_REVOKED"
	PreconditionAccountDisabled    = "ACCOUNT_DISABLED"
	PreconditionInvalidPassword    = "INVALID_PASSWORD"
)

// Metadata 定义所有强类型错误元数据结构体实现的契约：
//   - I18nVars 提供零反射的 {var} 模板插值变量；
//   - I18nSubkeys 提供按优先级排列的细化翻译子键（用于 details.<STATUS>.<subkey> 分层回落查表）。
//
// 实现方必须使用值接收者（因 oops.Context() 会自动解引用指针）。
type Metadata interface {
	I18nVars() map[string]string
	I18nSubkeys() []string
}

// ResourceInfo（对标 Google AIP-193 ResourceInfo）：描述不存在、冲突或受限的资源（404 / 409）。
type ResourceInfo struct {
	ResourceType string `json:"resource_type"`
	ResourceName string `json:"resource_name"`
	Field        string `json:"field,omitempty"`
}

func (m ResourceInfo) I18nVars() map[string]string {
	vars := map[string]string{
		"resource_type": m.ResourceType,
		"resource_name": m.ResourceName,
	}
	if m.Field != "" {
		vars["field"] = m.Field
	}
	return vars
}

func (m ResourceInfo) I18nSubkeys() []string {
	if m.ResourceType != "" && m.Field != "" {
		return []string{m.ResourceType + "." + m.Field, m.ResourceType}
	}
	if m.ResourceType != "" {
		return []string{m.ResourceType}
	}
	return nil
}

// PreconditionInfo（对标 Google AIP-193 PreconditionFailure）：描述前置状态、凭证或策略不满足（400 / 401 / 403）。
type PreconditionInfo struct {
	Type    string `json:"type"`
	Subject string `json:"subject,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

func (m PreconditionInfo) I18nVars() map[string]string {
	vars := map[string]string{
		"type": m.Type,
	}
	if m.Subject != "" {
		vars["subject"] = m.Subject
	}
	if m.Detail != "" {
		vars["detail"] = m.Detail
	}
	return vars
}

func (m PreconditionInfo) I18nSubkeys() []string {
	if m.Type == "" {
		return nil
	}
	return []string{m.Type}
}

// RetryInfo（对标 Google AIP-193 RetryInfo）：描述上游限流与建议退避重试信息（429）。
type RetryInfo struct {
	Service    string `json:"service"`
	RetryAfter string `json:"retry_after,omitempty"`
}

func (m RetryInfo) I18nVars() map[string]string {
	vars := map[string]string{
		"service": m.Service,
	}
	if m.RetryAfter != "" {
		vars["retry_after"] = m.RetryAfter
	}
	return vars
}

func (m RetryInfo) I18nSubkeys() []string {
	if m.Service == "" {
		return nil
	}
	return []string{m.Service}
}

// ValidationInfo 描述字段级校验失败汇总（400 Bad Request），具体字段错误位于 Problem.Errors。
type ValidationInfo struct {
	ErrorCount int `json:"error_count"`
}

func (m ValidationInfo) I18nVars() map[string]string {
	return map[string]string{
		"error_count": strconv.Itoa(m.ErrorCount),
	}
}

func (ValidationInfo) I18nSubkeys() []string {
	return []string{"validation"}
}

// DecodeInfo 描述请求体 JSON 解码失败元信息（400 Bad Request）。
type DecodeInfo struct {
	Reason string `json:"reason"`
	Offset int64  `json:"offset,omitempty"`
}

func (m DecodeInfo) I18nVars() map[string]string {
	vars := map[string]string{
		"reason": m.Reason,
	}
	if m.Offset > 0 {
		vars["offset"] = strconv.FormatInt(m.Offset, 10)
	}
	return vars
}

func (DecodeInfo) I18nSubkeys() []string {
	return []string{"decode"}
}

// MediaTypeInfo 描述 PATCH 文档非法或媒体类型不支持（400 Bad Request / 415 Unsupported Media Type）。
type MediaTypeInfo struct {
	ContentType string   `json:"content_type"`
	Supported   []string `json:"supported,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Field       string   `json:"field,omitempty"`
}

func (m MediaTypeInfo) I18nVars() map[string]string {
	vars := map[string]string{
		"content_type": m.ContentType,
	}
	if len(m.Supported) > 0 {
		vars["supported"] = strings.Join(m.Supported, ", ")
	}
	if m.Reason != "" {
		vars["reason"] = m.Reason
	}
	if m.Field != "" {
		vars["field"] = m.Field
	}
	return vars
}

func (MediaTypeInfo) I18nSubkeys() []string {
	return []string{"patch"}
}

// MethodInfo 描述路由不支持当前 HTTP 方法（405 Method Not Allowed）。
type MethodInfo struct {
	Method  string   `json:"method"`
	Path    string   `json:"path"`
	Allowed []string `json:"allowed,omitempty"`
}

func (m MethodInfo) I18nVars() map[string]string {
	vars := map[string]string{
		"method": m.Method,
		"path":   m.Path,
	}
	if len(m.Allowed) > 0 {
		vars["allowed"] = strings.Join(m.Allowed, ", ")
	}
	return vars
}

func (MethodInfo) I18nSubkeys() []string { return nil }

// RequestLimitInfo 描述请求体超过最大字节数上限（413 Content Too Large）。
type RequestLimitInfo struct {
	MaxBytes int64 `json:"max_bytes"`
}

func (m RequestLimitInfo) I18nVars() map[string]string {
	return map[string]string{
		"max_bytes": strconv.FormatInt(m.MaxBytes, 10),
	}
}

func (m RequestLimitInfo) I18nSubkeys() []string {
	if m.MaxBytes > 0 {
		return []string{"body_limit"}
	}
	return nil
}

// DecodeMetadata 将 Problem.Metadata 解码为强类型结构体 T（同时兼容进程内直接赋值与 JSON 反序列化后的 map[string]any）。
func DecodeMetadata[T any](p Problem) (T, error) {
	var zero T
	if p.Metadata == nil {
		return zero, fmt.Errorf("httperr: problem %d has no metadata", p.Status)
	}
	if v, ok := p.Metadata.(T); ok {
		return v, nil
	}
	if ptr, ok := p.Metadata.(*T); ok && ptr != nil {
		return *ptr, nil
	}
	raw, err := json.Marshal(p.Metadata)
	if err != nil {
		return zero, err
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return zero, err
	}
	return out, nil
}
