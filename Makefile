.PHONY: swag ent build test

swag: ## 格式化注解并生成 static/openapi/openapi.yaml
	go tool swag fmt
	go tool swag init -g cmd/main.go --output static/openapi --outputTypes yaml
	mv static/openapi/swagger.yaml static/openapi/openapi.yaml

ent: ## 重新生成 ent ORM 代码（修改 ent/schema/*.go 后执行）
	cd ent && go generate

build: ## 编译
	go build ./...

test: ## 运行测试
	go test ./...
