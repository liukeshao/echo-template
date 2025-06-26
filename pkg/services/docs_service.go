package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/ent"
	"gopkg.in/yaml.v3"
)

// DocsService 文档服务
type DocsService struct {
	orm    *ent.Client
	config *config.Config
	// 缓存已解析的文件，避免重复解析
	resolvedFiles map[string]map[string]interface{}
}

// NewDocsService 创建文档服务实例
func NewDocsService(orm *ent.Client, config *config.Config) *DocsService {
	return &DocsService{
		orm:           orm,
		config:        config,
		resolvedFiles: make(map[string]map[string]interface{}),
	}
}

// GenerateOpenAPISpec 生成OpenAPI规范
func (s *DocsService) GenerateOpenAPISpec(ctx context.Context) (interface{}, error) {
	// 读取并解析YAML文件（包含引用解析）
	spec, err := s.loadAndResolveOpenAPISpec()
	if err != nil {
		return nil, fmt.Errorf("加载OpenAPI规范失败: %w", err)
	}

	// 动态更新服务器信息
	if servers, ok := spec["servers"].([]interface{}); ok && len(servers) > 0 {
		if server, ok := servers[0].(map[string]interface{}); ok {
			server["url"] = s.config.App.Host
		}
	}

	// 动态更新文档标题
	if info, ok := spec["info"].(map[string]interface{}); ok {
		info["title"] = s.config.App.Docs.Title
	}

	return spec, nil
}

// loadAndResolveOpenAPISpec 从YAML文件加载并解析OpenAPI规范
func (s *DocsService) loadAndResolveOpenAPISpec() (map[string]interface{}, error) {
	// 查找主YAML文件路径
	yamlPaths := []string{
		"openapi/openapi.yaml",
		"../openapi/openapi.yaml",
		"../../openapi/openapi.yaml",
	}

	var yamlPath string
	for _, path := range yamlPaths {
		if _, err := os.Stat(path); err == nil {
			yamlPath = path
			break
		}
	}

	if yamlPath == "" {
		return nil, fmt.Errorf("找不到OpenAPI规范文件，查找路径: %v", yamlPaths)
	}

	// 获取绝对路径和基目录
	absPath, err := filepath.Abs(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("获取绝对路径失败: %w", err)
	}

	baseDir := filepath.Dir(absPath)

	// 清空缓存
	s.resolvedFiles = make(map[string]map[string]interface{})

	// 加载并解析主文件
	spec, err := s.loadFileWithResolution(absPath, baseDir)
	if err != nil {
		return nil, fmt.Errorf("解析主文件失败: %w", err)
	}

	return spec, nil
}

// loadFileWithResolution 加载文件并解析其中的引用
func (s *DocsService) loadFileWithResolution(filePath, baseDir string) (map[string]interface{}, error) {
	// 检查缓存
	if cached, exists := s.resolvedFiles[filePath]; exists {
		return cached, nil
	}

	// 读取文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败 %s: %w", filePath, err)
	}

	// 解析YAML
	var spec map[string]interface{}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("解析YAML失败: %w", err)
	}

	// 解析引用，传递当前文档作为上下文
	resolvedInterface, err := s.resolveReferencesWithContext(spec, baseDir, spec)
	if err != nil {
		return nil, fmt.Errorf("解析引用失败: %w", err)
	}

	// 类型断言
	resolvedSpec, ok := resolvedInterface.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("解析后的规范不是有效的对象类型")
	}

	// 缓存结果
	s.resolvedFiles[filePath] = resolvedSpec

	return resolvedSpec, nil
}

// resolveReferences 递归解析对象中的所有引用
func (s *DocsService) resolveReferences(obj interface{}, baseDir string) (interface{}, error) {
	return s.resolveReferencesWithContext(obj, baseDir, nil)
}

// resolveReferencesWithContext 递归解析对象中的所有引用，带上下文
func (s *DocsService) resolveReferencesWithContext(obj interface{}, baseDir string, rootDoc map[string]interface{}) (interface{}, error) {
	switch v := obj.(type) {
	case map[string]interface{}:
		// 检查是否包含 $ref
		if refValue, hasRef := v["$ref"]; hasRef {
			if refStr, ok := refValue.(string); ok {
				resolved, err := s.resolveReferenceWithContext(refStr, baseDir, rootDoc)
				if err != nil {
					return nil, err
				}
				return resolved, nil
			}
		}

		// 递归处理 map 中的所有值
		result := make(map[string]interface{})
		for key, value := range v {
			resolved, err := s.resolveReferencesWithContext(value, baseDir, rootDoc)
			if err != nil {
				return nil, err
			}
			result[key] = resolved
		}
		return result, nil

	case []interface{}:
		// 递归处理数组中的所有元素
		result := make([]interface{}, len(v))
		for i, item := range v {
			resolved, err := s.resolveReferencesWithContext(item, baseDir, rootDoc)
			if err != nil {
				return nil, err
			}
			result[i] = resolved
		}
		return result, nil

	default:
		// 基本类型直接返回
		return obj, nil
	}
}

// resolveReference 解析单个引用
func (s *DocsService) resolveReference(ref, baseDir string) (interface{}, error) {
	return s.resolveReferenceWithContext(ref, baseDir, nil)
}

// resolveReferenceWithContext 解析单个引用，带上下文
func (s *DocsService) resolveReferenceWithContext(ref, baseDir string, rootDoc map[string]interface{}) (interface{}, error) {
	// 解析引用路径
	var filePath, jsonPath string

	if strings.HasPrefix(ref, "#/") {
		// 本地引用，在当前文档中查找
		if rootDoc == nil {
			return nil, fmt.Errorf("本地引用 %s 缺少文档上下文", ref)
		}
		jsonPath = strings.TrimPrefix(ref, "#/")
		result, err := s.resolveJSONPath(rootDoc, jsonPath)
		if err != nil {
			return nil, fmt.Errorf("解析本地引用失败 %s: %w", ref, err)
		}
		return result, nil
	} else if strings.Contains(ref, "#/") {
		// 外部文件引用
		parts := strings.SplitN(ref, "#/", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("无效的引用格式: %s", ref)
		}
		filePath = parts[0]
		jsonPath = parts[1]
	} else {
		// 整个文件引用
		filePath = ref
		jsonPath = ""
	}

	// 解析文件路径
	var targetPath string
	if filepath.IsAbs(filePath) {
		targetPath = filePath
	} else {
		targetPath = filepath.Join(baseDir, filePath)
	}

	// 加载目标文件
	targetSpec, err := s.loadFileWithResolution(targetPath, filepath.Dir(targetPath))
	if err != nil {
		return nil, fmt.Errorf("加载引用文件失败 %s: %w", targetPath, err)
	}

	// 如果没有 JSON 路径，返回整个文件
	if jsonPath == "" {
		return targetSpec, nil
	}

	// 解析 JSON 路径
	result, err := s.resolveJSONPath(targetSpec, jsonPath)
	if err != nil {
		return nil, fmt.Errorf("解析JSON路径失败 %s 在文件 %s: %w", jsonPath, targetPath, err)
	}

	return result, nil
}

// resolveJSONPath 解析 JSON 路径
func (s *DocsService) resolveJSONPath(data interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, "/")
	current := data

	for _, part := range parts {
		if part == "" {
			continue
		}

		switch v := current.(type) {
		case map[string]interface{}:
			if next, exists := v[part]; exists {
				current = next
			} else {
				return nil, fmt.Errorf("路径 '%s' 在对象中不存在", part)
			}
		case []interface{}:
			// 处理数组索引（如果需要的话）
			return nil, fmt.Errorf("暂不支持数组索引路径: %s", part)
		default:
			return nil, fmt.Errorf("无法在类型 %T 中解析路径 '%s'", current, part)
		}
	}

	return current, nil
}

// IsDocsEnabled 检查文档是否启用
func (s *DocsService) IsDocsEnabled() bool {
	return s.config.App.Docs.Enabled
}

// GetOpenAPIYamlPath 获取OpenAPI YAML文件的路径（用于开发和调试）
func (s *DocsService) GetOpenAPIYamlPath() (string, error) {
	yamlPaths := []string{
		"docs/openapi.yaml",
		"../docs/openapi.yaml",
		"../../docs/openapi.yaml",
		"openapi.yaml",
	}

	for _, path := range yamlPaths {
		if absPath, err := filepath.Abs(path); err == nil {
			if _, err := os.Stat(absPath); err == nil {
				return absPath, nil
			}
		}
	}

	return "", os.ErrNotExist
}
