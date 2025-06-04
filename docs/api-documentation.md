# API 文档集成指南

本项目已集成 [Stoplight Elements](https://github.com/stoplightio/elements) 来提供美观的 API 文档。

## 功能特性

- 🚀 基于 OpenAPI 3.0.3 规范
- 🎨 现代化的文档界面，基于 Stoplight Elements
- 🔒 环境安全控制，生产环境自动禁用
- 📱 响应式设计，支持移动设备
- 🔍 交互式 API 测试（Try It）
- 📝 自动生成的 API 规范

## 访问方式

### 开发环境
启动应用后，访问：http://localhost:8000/docs

### 生产环境
生产环境下文档功能被**完全禁用**，访问文档路径将返回 404 错误。

## 配置说明

### 文档配置

在 `config.yaml` 中配置文档功能：

```yaml
app:
  docs:
    enabled: true      # 是否启用文档，生产环境请设置为 false
    path: "/docs"      # 文档访问路径
    title: "Echo Template API Documentation"  # 文档标题
```

### 环境变量控制

也可以通过环境变量控制：

```bash
export ECHO_TEMPLATE_APP_DOCS_ENABLED=false
export ECHO_TEMPLATE_APP_DOCS_PATH="/docs"
export ECHO_TEMPLATE_APP_DOCS_TITLE="My API Documentation"
```

## 安全特性

### 生产环境保护
- 生产环境下文档功能被完全禁用
- 访问文档路径返回标准 404 错误，不暴露文档存在
- 所有文档访问都会被记录到日志中

### 访问控制
- 检查配置中的 `docs.enabled` 设置
- 验证当前运行环境
- 记录所有文档访问日志（包括 IP、User-Agent 等）

### 安全头
文档页面自动添加安全响应头：
- `X-Frame-Options: DENY`
- `X-Content-Type-Options: nosniff`
- `X-XSS-Protection: 1; mode=block`
- `Referrer-Policy: strict-origin-when-cross-origin`
- Content Security Policy（允许 unpkg.com）

## API 规范

### OpenAPI 规范文件
可以直接访问 OpenAPI 规范：http://localhost:8000/openapi.json

### 规范结构
当前包含以下 API 模块：
- 认证模块（注册、登录）
- 用户管理模块

## 扩展文档

### 添加新的 API 文档

现在API文档使用YAML文件进行管理，更容易维护和编辑：

1. **编辑OpenAPI规范文件**: `docs/openapi.yaml`
2. **按照OpenAPI 3.1规范格式添加新的接口定义**
3. **应用会自动加载更新后的YAML文件**

#### YAML文件结构
```yaml
openapi: 3.1.0
info:
  title: Echo Template API Documentation
  description: 基于Echo框架的高性能Web应用API文档
  version: 1.0.0

paths:
  /api/v1/your-endpoint:
    post:
      tags:
        - 标签名
      summary: 接口摘要
      description: 接口详细描述
      operationId: operationName
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/YourSchema'
      responses:
        '200':
          description: 成功响应
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ResponseSchema'

components:
  schemas:
    YourSchema:
      type: object
      title: 你的数据模型
      properties:
        field_name:
          type: string
          description: 字段描述
```

#### 动态配置
应用会自动从配置文件中读取以下信息并更新到API规范中：
- **服务器地址**: 使用 `app.host` 配置
- **文档标题**: 使用 `app.docs.title` 配置

### 维护和验证

#### 验证YAML语法
```bash
# 使用在线工具验证OpenAPI规范
# 或使用本地工具如swagger-codegen验证
go run cmd/web/main.go
curl http://localhost:8000/openapi.json | jq .
```

#### 常见编辑场景

1. **添加新接口**:
   - 在 `paths` 部分添加新的路径
   - 在 `components/schemas` 添加相关数据模型

2. **添加新的数据模型**:
   - 在 `components/schemas` 部分添加新的schema定义
   - 使用 `$ref` 引用已定义的模型

3. **添加新的响应类型**:
   - 在 `components/responses` 部分添加新的响应定义

### 自定义文档页面

可以修改 `pkg/handlers/docs_handler.go` 中的 `GetDocsPage` 方法来自定义：
- 页面标题和样式
- Elements 组件配置
- 添加自定义 JavaScript 逻辑

## 部署注意事项

### 生产环境
1. 确保 `config.prod.yaml` 中设置 `docs.enabled: false`
2. 或设置环境变量 `ECHO_TEMPLATE_APP_DOCS_ENABLED=false`
3. 检查日志中是否有文档访问记录

### 开发/测试环境
1. 设置 `docs.enabled: true`
2. 根据需要调整文档标题和路径
3. 确保防火墙允许访问文档端口

## 故障排除

### 文档无法访问
1. 检查 `docs.enabled` 配置
2. 确认当前环境不是生产环境
3. 查看应用日志中的错误信息

### 样式异常
1. 检查网络连接，确保能访问 unpkg.com
2. 检查 CSP 设置是否正确
3. 查看浏览器控制台错误

### OpenAPI 规范错误
1. 检查 `docs_service.go` 中的 JSON 格式
2. 使用在线工具验证 OpenAPI 规范格式
3. 查看应用日志中的生成错误

## 相关链接

- [Stoplight Elements 官方文档](https://github.com/stoplightio/elements)
- [OpenAPI 3.0.3 规范](https://spec.openapis.org/oas/v3.0.3)
- [Echo 框架文档](https://echo.labstack.com/) 