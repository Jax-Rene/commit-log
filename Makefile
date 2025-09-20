run:
	go run cmd/server/main.go 

dev:
	@echo "启动开发模式..."
	@echo "Gin 会自动重载模板文件"
	GIN_MODE=debug go run cmd/server/main.go

test:
	go test -v ./...

generate-test-data:
	go run scripts/generate_test_data.go