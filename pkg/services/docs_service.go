package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/liukeshao/echo-template/config"
	"github.com/liukeshao/echo-template/ent"
	"gopkg.in/yaml.v3"
)

// DocsService 文档服务
type DocsService struct {
	orm    *ent.Client
	config *config.Config
}

// NewDocsService 创建文档服务实例
func NewDocsService(orm *ent.Client, config *config.Config) *DocsService {
	return &DocsService{
		orm:    orm,
		config: config,
	}
}

// GenerateOpenAPISpec 生成OpenAPI规范
func (s *DocsService) GenerateOpenAPISpec(ctx context.Context) (interface{}, error) {
	// 读取YAML文件
	spec, err := s.loadOpenAPISpec()
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

// loadOpenAPISpec 从YAML文件加载OpenAPI规范
func (s *DocsService) loadOpenAPISpec() (map[string]interface{}, error) {
	// 查找YAML文件路径
	yamlPaths := []string{
		"docs/openapi.yaml",
		"../docs/openapi.yaml",
		"../../docs/openapi.yaml",
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

	// 获取绝对路径
	absPath, err := filepath.Abs(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("获取绝对路径失败: %w", err)
	}

	// 读取YAML文件
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败 %s: %w", absPath, err)
	}

	// 解析YAML
	var spec map[string]interface{}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("解析YAML失败: %w", err)
	}

	return spec, nil
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
