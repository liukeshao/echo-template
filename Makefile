.PHONY: swag ent build test cover

swag: ## 格式化注解并生成 OpenAPI 3.1 规范 static/openapi/openapi.yaml
	go tool swag fmt
	GOFLAGS=-buildvcs=false go tool swag init --v3.1 -d cmd,pkg/handlers,pkg/types -g main.go --output static/openapi --outputTypes yaml
	mv static/openapi/swagger.yaml static/openapi/openapi.yaml

ent: ## 重新生成 ent ORM 代码（修改 ent/schema/*.go 后执行）
	cd ent && go generate

build: ## 编译
	go build ./...

test: ## 运行测试（启用竞态检测）
	go test -race ./...

cover: ## 统计业务包单元测试覆盖率（排除 ent 生成代码与 testutil）
	go test -race -coverprofile=coverage.out $$(go list ./config/... ./pkg/... | grep -v '/pkg/testutil$$')
	go tool cover -func=coverage.out | tail -n 1
	rm -f coverage.out
