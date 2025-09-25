# Echo Template

## 项目介绍

Echo Template 是一个基于 Go 语言和 Echo 框架构建的现代化 Web 应用程序模板。该项目提供了完整的用户认证、权限管理、组织架构等企业级功能，可以作为快速构建 Web 应用的起点。

## 主要特性

### 🔐 认证与授权
- **JWT 认证**：支持 Access Token 和 Refresh Token 机制
- **用户管理**：完整的用户注册、登录、登出功能

### 📊 数据管理
- **ORM 框架**：使用 Ent 提供类型安全的数据访问
- **数据库迁移**：自动化的数据库 schema 管理
- **软删除**：支持逻辑删除机制
- **审计日志**：完整的创建、更新、删除时间记录

### 🔧 工程化特性
- **配置管理**：基于 Viper 的配置系统
- **请求验证**：使用 Zog 进行输入验证
- **错误处理**：标准化的错误响应格式
- **日志记录**：结构化日志输出
- **优雅关闭**：支持服务器优雅关闭

## 技术栈

### 后端技术
- **Go**：主要开发语言
- **Echo**：高性能 Web 框架
- **Ent**：类型安全的 ORM 框架
- **JWT**：JSON Web Token 认证
- **Viper**：配置管理
- **Zog**：请求验证
- **ULID**：分布式 ID 生成
- **SQLite**：轻量级数据库

### 开发工具
- **OpenAPI 3.0**：API 文档规范
- **Redocly**：API 文档工具
- **Makefile**：构建脚本
- **Git Hook**：代码质量检查

## 项目结构

```
echo-template/
├── api-specs/                 # API 规范文档
│   ├── openapi/              # OpenAPI 规范文件
│   └── docs/                 # 生成的文档
├── cmd/web/                  # 应用程序入口
├── config/                   # 配置文件
├── ent/                      # Ent ORM 生成代码
│   └── schema/               # 数据模型定义
├── pkg/                      # 核心业务逻辑
│   ├── handlers/             # HTTP 处理器
│   ├── services/             # 业务服务层
│   ├── middleware/           # 中间件
│   ├── types/                # 数据类型定义
│   └── utils/                # 工具函数
├── Makefile                  # 构建脚本
├── go.mod                    # Go 模块定义
└── README.md                 # 项目说明
```

## 快速开始

### 环境要求

- Go 1.25 或更高版本
- Node.js 16+ （用于 API 文档）
- SQLite3

### 安装依赖

```bash
# 克隆项目
git clone https://github.com/liukeshao/echo-template.git
cd echo-template

# 安装 Go 依赖
go mod download

# 安装 Ent 代码生成工具
make ent-install

# 生成 Ent 代码
make ent-gen

# 安装 API 文档依赖
cd api-specs
npm install
cd ..
```

### 配置应用

主要配置项：

```toml
[http]
port = 8000

[app]
name = "echo-template"
host = "http://localhost:8000"
environment = "local"

[jwt]
secret = "your-super-secret-jwt-key-change-this-in-production"
accessTokenExpiry = "24h"
refreshTokenExpiry = "168h"

[database]
driver = "sqlite3"
connection = "dbs/main.db?_journal=WAL&_timeout=5000&_fk=true"
```

### 运行应用

```bash
# 启动开发服务器
make run

# 或者直接运行
go run cmd/web/main.go
```

服务器将在 `http://localhost:8000` 启动。

### 运行测试

```bash
# 运行所有测试
make test

# 检查依赖更新
make check-updates
```

## API 文档

### 生成文档

```bash
cd api-specs

# 启动文档预览服务器
npm start

# 构建文档
npm run build

# 验证 API 规范
npm test
```

## 开发指南

### 添加新实体

```bash
# 创建新的 Ent 实体
make ent-new name=MyEntity

# 修改 ent/schema/myentity.go 文件

# 重新生成代码
make ent-gen
```

### 添加新的 API 端点

1. 在 `pkg/types/` 中定义请求和响应类型
2. 在 `pkg/services/` 中实现业务逻辑
3. 在 `pkg/handlers/` 中创建 HTTP 处理器
4. 在 `api-specs/` 中添加 API 文档

### 数据库迁移

```bash
# 生成迁移文件
go run ent/migrate/main.go

# 应用迁移
go run cmd/web/main.go
```

## 部署

### 构建生产版本

```bash
# 构建二进制文件
go build -o bin/echo-template cmd/web/main.go

# 运行
./bin/echo-template
```

## 贡献指南

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/new-feature`)
3. 提交更改 (`git commit -am 'Add new feature'`)
4. 推送分支 (`git push origin feature/new-feature`)
5. 创建 Pull Request

## 许可证

本项目使用 MIT 许可证。详细信息请参阅 [LICENSE](LICENSE) 文件。

## 联系方式

如有问题或建议，请通过以下方式联系：

- 项目地址：https://github.com/liukeshao/echo-template
- 问题反馈：https://github.com/liukeshao/echo-template/issues

## 更新日志

### v1.0.0
- 基础用户认证系统
- OpenAPI 文档
- 完整的测试覆盖 