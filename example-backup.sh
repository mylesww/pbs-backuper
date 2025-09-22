#!/bin/bash

# PVE Backup Server Chunk Data Backup Script
# 这个脚本演示如何使用 backuper 工具进行 PVE 备份服务器的 chunk 数据备份

# ==================== 配置部分 ====================

# PVE backup server 的 .chunk 目录路径
CHUNK_PATH="/var/lib/vz/backup/.chunks"

# 远程存储路径（需要先配置 rclone）
REMOTE_PATH="backup-remote:pve/chunks"

# 临时文件存储路径
TEMP_PATH="/tmp/backuper"

# rclone 二进制文件路径
RCLONE_BINARY="rclone"

# rclone 配置文件路径
RCLONE_CONFIG="/root/.config/rclone/rclone.conf"

# 日志文件路径
LOG_PATH="/var/log/backuper.log"

# 前缀位数（用于分组）- 只在全量备份时有效
PREFIX_DIGITS=2

# rclone 额外参数
RCLONE_ARGS="--transfers=4,--checkers=8,--progress"

# 超时时间
TIMEOUT="2h"

# ==================== 函数定义 ====================

# 检查依赖
check_dependencies() {
    echo "正在检查依赖..."
    
    # 检查 backuper 可执行文件
    if [ ! -f "./backuper" ]; then
        echo "错误: 找不到 backuper 可执行文件"
        exit 1
    fi
    
    # 检查 rclone
    if ! command -v "$RCLONE_BINARY" &> /dev/null; then
        echo "错误: 找不到 rclone，请确保已安装并配置"
        exit 1
    fi
    
    # 检查 chunk 目录
    if [ ! -d "$CHUNK_PATH" ]; then
        echo "错误: chunk 目录不存在: $CHUNK_PATH"
        exit 1
    fi
    
    echo "依赖检查通过"
}

# 全量备份
full_backup() {
    echo "开始全量备份..."
    ./backuper full \
        --chunk-path "$CHUNK_PATH" \
        --remote-path "$REMOTE_PATH" \
        --temp-path "$TEMP_PATH" \
        --rclone-binary "$RCLONE_BINARY" \
        --rclone-config "$RCLONE_CONFIG" \
        --rclone-args "$RCLONE_ARGS" \
        --log-path "$LOG_PATH" \
        --prefix-digits "$PREFIX_DIGITS" \
        --timeout "$TIMEOUT" \
        --verbose
}

# 增量备份
incremental_backup() {
    echo "开始增量备份..."
    ./backuper incremental \
        --chunk-path "$CHUNK_PATH" \
        --remote-path "$REMOTE_PATH" \
        --temp-path "$TEMP_PATH" \
        --rclone-binary "$RCLONE_BINARY" \
        --rclone-config "$RCLONE_CONFIG" \
        --rclone-args "$RCLONE_ARGS" \
        --log-path "$LOG_PATH" \
        --timeout "$TIMEOUT" \
        --verbose
}

# 显示帮助
show_help() {
    echo "PVE Backup Server Chunk Data Backup Script"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  full         执行全量备份"
    echo "  incremental  执行增量备份"
    echo "  auto         自动模式（周日全量，其他增量）"
    echo "  help         显示此帮助信息"
    echo ""
    echo "示例:"
    echo "  $0 full         # 执行全量备份"
    echo "  $0 incremental  # 执行增量备份"
    echo "  $0 auto         # 自动模式"
}

# ==================== 主程序 ====================

# 创建日志目录
mkdir -p "$(dirname "$LOG_PATH")"

# 检查参数
if [ $# -eq 0 ]; then
    echo "错误: 需要指定备份模式"
    show_help
    exit 1
fi

# 检查依赖
check_dependencies

# 根据参数执行相应操作
case "$1" in
    "full")
        full_backup
        ;;
    "incremental")
        incremental_backup
        ;;
    "auto")
        # 自动模式：周日执行全量备份，其他时间执行增量备份
        DAY_OF_WEEK=$(date +%u)
        if [ "$DAY_OF_WEEK" -eq 7 ]; then
            echo "今天是周日，执行全量备份"
            full_backup
        else
            echo "执行增量备份"
            incremental_backup
        fi
        ;;
    "help"|"-h"|"--help")
        show_help
        ;;
    *)
        echo "错误: 未知选项 '$1'"
        show_help
        exit 1
        ;;
esac

# 检查执行结果
if [ $? -eq 0 ]; then
    echo "备份完成成功！"
    echo "日志文件: $LOG_PATH"
else
    echo "备份失败，请检查日志文件: $LOG_PATH"
    exit 1
fi