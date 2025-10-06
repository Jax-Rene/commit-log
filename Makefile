# 本地快速开发 Debug 模式
dev:
	@echo "启动开发模式..."
	@echo "Gin 会自动重载模板文件"
	rm -rf ./web/static/dist
	npm run build
	GIN_MODE=debug go run cmd/server/main.go

# 测试模式，运行单元测试
test:
	go test -v ./...

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
