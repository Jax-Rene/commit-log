GOCACHE ?= $(CURDIR)/.cache/go-build
GO_FILES := $(shell find cmd internal scripts tests -type f -name '*.go' 2>/dev/null)

.PHONY: build test lint fix run deploy generate-test-data docker-build docker-dev docker-dev-down \
	fly-init fly-deploy fly-status fly-logs fly-ssh fly-sync-product-data create-pr

# 统一构建：Go + 前端资源
build:
	go mod tidy
	go build ./...
	pnpm run build

# 测试模式，运行全部 Go/前端测试
test:
	GOCACHE=$(GOCACHE) go test -v ./...
	pnpm test

# 统一 lint：后端 + 前端
lint:
	@unformatted="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$unformatted" ]; then \
		echo "以下 Go 文件未经过 gofmt:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --config .golangci.yml ./...; \
	else \
		go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run --config .golangci.yml ./...; \
	fi
	pnpm run lint

# 自动修复格式与可修复 lint
fix:
	gofmt -w $(GO_FILES)
	pnpm run lint:fix
	pnpm run format

run:
	GIN_MODE=release go run cmd/server/main.go

deploy: fly-deploy

# 生成测试数据
generate-test-data:
	go run scripts/generate_test_data.go

# 生产环境构建：docker 编译，主要用于模拟生产环境
docker-build:
	docker compose -f docker-compose.dev.yml build

# 生产环境运行：docker 运行，主要用于模拟生产环境运行
docker-dev:
	docker compose -f docker-compose.dev.yml up

# 生产环境关闭：docker 关闭，主要用于模拟生产环境关闭
docker-dev-down:
	docker compose -f docker-compose.dev.yml down

# fly.io 部署相关命令
fly-init: # 初始化 fly.io 配置
	fly launch --now

fly-deploy: # 部署到 fly.io
	fly deploy

fly-status: # 查看 fly.io 状态
	fly status

fly-logs: # 查看 fly.io 日志
	fly logs

fly-ssh: # 通过 ssh 连接到 fly.io 实例
	fly ssh console

fly-sync-product-data: # 同步线上数据到本地开发使用
	rm -rf commitlog.db.backup uploads
	fly ssh sftp -a commitlog get /data/commitlog.db.backup
	mv commitlog.db.backup commitlog.db
	fly ssh sftp -a commitlog get -R /data/uploads
	mv uploads/* ./web/static/uploads

# 利用 GitHub CLI 自动创建 Pull Request
create-pr:
	gh pr create --fill
