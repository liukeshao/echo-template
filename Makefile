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
