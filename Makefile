.PHONY: api ent build test cover

api: ## 编译 TypeSpec 生成 OpenAPI 3.1 (static/openapi/openapi.yaml) 与 Go 协议代码 (pkg/api/*.gen.go)
	rm -f pkg/api/*.gen.go
	cd api && ./node_modules/.bin/tsp compile spec/main.tsp
	gofmt -w pkg/api/

ent: ## 重新生成 ent ORM 代码（修改 ent/schema/*.go 后执行）
	cd ent && go generate

build: ## 编译
	go build ./...

test: ## 运行测试（启用竞态检测）
	go test -race ./...

cover: ## 统计业务包单元测试覆盖率（排除 ent 生成代码与 testutil）
	go test -race -coverprofile=coverage.out $$(go list ./config/... ./pkg/... ./internal/... | grep -v '/internal/testutil$$')
	go tool cover -func=coverage.out | tail -n 1
	rm -f coverage.out
