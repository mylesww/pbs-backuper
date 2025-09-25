.PHONY: build test clean install help

# 默认目标
help: ## 显示帮助信息
	@echo "可用目标:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# 构建
build: ## 构建可执行文件
	go build -o pbs-backuper .

# 构建优化版本
build-release: ## 构建发布版本（优化）
	CGO_ENABLED=0 go build -ldflags="-w -s" -o pbs-backuper .

# 运行测试
test: ## 运行所有测试
	go test -v ./...

# 运行测试并生成覆盖率报告
test-coverage: ## 运行测试并生成覆盖率报告
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# 清理
clean: ## 清理构建文件
	rm -f pbs-backuper
	rm -f coverage.out coverage.html

# 安装依赖
deps: ## 安装/更新依赖
	go mod download
	go mod tidy

# 代码格式化
fmt: ## 格式化代码
	go fmt ./...

# 代码检查
lint: ## 运行代码检查
	golangci-lint run

# 安装到系统
install: build ## 安装到系统PATH
	sudo cp pbs-backuper /usr/local/bin/

# 卸载
uninstall: ## 从系统中卸载
	sudo rm -f /usr/local/bin/pbs-backuper

# 创建示例配置
example-config: ## 创建示例配置文件
	@echo "创建示例脚本..."
	@cat > example-backup.sh << 'EOF'
#!/bin/bash

# PVE Backup Server Chunk Backup Script
# 配置变量
CHUNK_PATH="/var/lib/vz/backup/.chunks"
REMOTE_PATH="backup-remote:pve/chunks"
TEMP_PATH="/tmp/pbs-backuper"
RCLONE_BINARY="rclone"
RCLONE_CONFIG="/root/.config/rclone/rclone.conf"
LOG_PATH="/var/log/pbs-backuper.log"

# 全量备份（每周日）
if [ $$(date +%u) -eq 7 ]; then
    echo "Running full backup..."
    ./pbs-backuper full \
        --chunk-path "$$CHUNK_PATH" \
        --remote-path "$$REMOTE_PATH" \
        --temp-path "$$TEMP_PATH" \
        --rclone-binary "$$RCLONE_BINARY" \
        --rclone-config "$$RCLONE_CONFIG" \
        --log-path "$$LOG_PATH" \
        --prefix-digits 2 \
        --verbose
else
    echo "Running incremental backup..."
    ./pbs-backuper incremental \
        --chunk-path "$$CHUNK_PATH" \
        --remote-path "$$REMOTE_PATH" \
        --temp-path "$$TEMP_PATH" \
        --rclone-binary "$$RCLONE_BINARY" \
        --rclone-config "$$RCLONE_CONFIG" \
        --log-path "$$LOG_PATH" \
        --verbose
fi
EOF
	@chmod +x example-backup.sh
	@echo "示例脚本已创建: example-backup.sh"

# 检查依赖
check-deps: ## 检查系统依赖
	@echo "检查依赖..."
	@command -v go >/dev/null 2>&1 || { echo "需要安装Go但未找到"; exit 1; }
	@command -v rclone >/dev/null 2>&1 || { echo "警告: 在PATH中未找到rclone"; }
	@echo "依赖检查完成"

# 所有操作
all: deps fmt test build ## 执行所有构建步骤

# 发布准备
release-prep: clean deps fmt test build-release test ## 准备发布版本