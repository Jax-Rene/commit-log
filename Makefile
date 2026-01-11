# 本地快速开发 Debug 模式
dev:
	@echo "启动开发模式..."
	@echo "Gin 会自动重载模板文件"
	rm -rf ./web/static/dist
	pnpm run build
	GIN_MODE=debug go run cmd/server/main.go

GOCACHE ?= $(CURDIR)/.cache/go-build

# 测试模式，运行全部 Go/前端测试
test:
	GOCACHE=$(GOCACHE) go test -v ./...
	pnpm test

# 生成测试数据
generate-test-data:
	go run scripts/generate_test_data.go

# 一次性迁移：补齐标签多语言展示（tag_translations）
migrate-tag-translations:
	go run ./scripts/migrate_tag_translations -db commitlog.db -langs zh,en

# 生产环境构建：docker 编译，主要用于模拟生产环境
docker-build:
	docker compose -f docker-compose.dev.yml build

# 生产环境运行：docker 运行，主要用于模拟生产环境运行
docker-dev:
	docker compose -f docker-compose.dev.yml up

# 生产环境关闭：docker 关闭，主要用于模拟生产环境关闭
docker-dev-down:
	docker compose -f docker-compose.dev.yml down

lint:
	staticcheck ./...

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
