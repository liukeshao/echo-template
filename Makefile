.PHONY: help
help: ## Print make targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: ent-install
ent-install: ## Install Ent code-generation module
	go get entgo.io/ent/cmd/ent

.PHONY: ent-gen
ent-gen: ## Generate Ent code
	go generate ./ent

.PHONY: ent-new
ent-new: ## Create a new Ent entity (ie, make ent-new name=MyEntity)
	go run entgo.io/ent/cmd/ent new $(name)

.PHONY: run
run: ## Run the application
	@echo "启动服务器..."
	go run cmd/web/main.go

.PHONY: test
test: ## Run all tests
	go test ./...

.PHONY: check-updates
check-updates: ## Check for direct dependency updates
	go list -u -m -f '{{if not .Indirect}}{{.}}{{end}}' all | grep "\["

# OpenAPI 文档相关命令
.PHONY: docs-lint
docs-lint: ## 校验 OpenAPI 规范
	@echo "🔍 校验 OpenAPI 规范..."
	@cd api-specs && npm test

.PHONY: docs-build
docs-build: ## 生成静态 HTML 文档到 api-specs
	@echo "🏗️  生成静态 HTML 文档..."
	@cd api-specs && redocly build-docs openapi/openapi.yaml --output docs/index.html
	@echo "✅ 文档生成完成: api-specs/docs/index.html"

.PHONY: docs-bundle
docs-bundle: ## 生成单文件 OpenAPI 规范
	@echo "📦 生成单文件 OpenAPI 规范..."
	@cd api-specs && npm run build
	@echo "✅ 规范包生成完成: api-specs/dist/bundle.yaml"

.PHONY: docs-clean
docs-clean: ## 清理生成的文档文件
	@echo "🧹 清理文档文件..."
	@rm -rf api-specs/docs api-specs/dist

.PHONY: docs-check
docs-check: docs-lint docs-build docs-bundle ## 完整的文档检查和构建
	@echo "✅ 文档检查和构建完成！"

.PHONY: docs
docs: docs-check ## docs-check 的别名
