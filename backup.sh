#!/bin/bash

# Comprehensive PVE Backup Server Script
# 这个脚本提供完整的 PVE 备份服务器数据备份功能
# 包含 chunk 数据备份（支持全量/增量）和其他目录的按时间备份

TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")                # 时间戳
DATE_FOLDER=$(date +"%Y-%m-%d")                       # 日期文件夹
# ==================== 配置部分 ====================

# 源目录配置
SOURCE_DIR="/mnt/datastore/rclone"                    # 主数据目录
CHUNK_PATH="$SOURCE_DIR/.chunks"                      # chunk 目录路径

# 远程存储配置
REMOTE_BASE="hlist:/pbs/001"                              # 远程存储基础路径
CHUNK_REMOTE_PATH="$REMOTE_BASE-chunk"               # chunk 远程存储路径

# 本地配置
TEMP_PATH="temp/temp_$TIMESTAMP"                      # 临时文件存储路径（带时间戳确保干净）
LOG_DIR="temp/log/$TIMESTAMP"                                     # 日志目录

# pbs-backuper 配置（用于 chunk 备份）
PBS_BACKUPER="./pbs-backuper"                         # pbs-backuper 可执行文件路径
PREFIX_DIGITS=2                                        # 前缀位数（用于分组）
TIMEOUT="2h"                                          # 超时时间

# rclone 配置
RCLONE_BINARY="rclone"                                # rclone 二进制文件路径
RCLONE_CONFIG="/Users/wlnxing/Library/CloudStorage/SynologyDrive-drive/sync/tools/rclone/config.txt"  # rclone 配置文件路径
RCLONE_ARGS="--transfers=4,--checkers=8,--progress"   # rclone 额外参数（用于 chunk 备份）

# 日志文件
CHUNK_LOG="$LOG_DIR/chunk_backup.log"     # chunk 备份日志
SYNC_LOG="$LOG_DIR/sync_log.log"          # 同步备份日志
MAIN_LOG="$LOG_DIR/comprehensive_backup.log"  # 主日志文件

# rclone 同步参数（用于非 chunk 目录备份）
RCLONE_SYNC_ARGS="-P --retries=8 --retries-sleep=60s --transfers=8"

# ==================== 函数定义 ====================

# 日志函数
log() {
    local message="$(date '+%Y-%m-%d %H:%M:%S') - $1"
    echo "$message"
    echo "$message" >> "$MAIN_LOG"
}

# 错误日志函数
log_error() {
    local message="$(date '+%Y-%m-%d %H:%M:%S') - ERROR: $1"
    echo "$message" >&2
    echo "$message" >> "$MAIN_LOG"
}

# 检查依赖
check_dependencies() {
    log "正在检查依赖..."
    
    # 检查源目录
    if [ ! -d "$SOURCE_DIR" ]; then
        log_error "源目录不存在: $SOURCE_DIR"
        exit 1
    fi
    
    # 检查 chunk 目录
    if [ ! -d "$CHUNK_PATH" ]; then
        log_error "chunk 目录不存在: $CHUNK_PATH"
        exit 1
    fi
    
    # 检查 pbs-backuper 可执行文件
    if [ ! -f "$PBS_BACKUPER" ]; then
        log_error "找不到 pbs-backuper 可执行文件: $PBS_BACKUPER"
        exit 1
    fi
    
    # 检查 rclone
    if ! command -v "$RCLONE_BINARY" &> /dev/null; then
        log_error "找不到 rclone，请确保已安装并配置"
        exit 1
    fi
    
    # 检查 rclone 配置文件
    if [ ! -f "$RCLONE_CONFIG" ]; then
        log_error "rclone 配置文件不存在: $RCLONE_CONFIG"
        exit 1
    fi
    
    log "依赖检查通过"
}

# 创建必要的目录
create_directories() {
    log "创建必要的目录..."
    
    # 创建日志目录
    mkdir -p "$LOG_DIR"
    
    # 创建临时目录
    mkdir -p "$TEMP_PATH"
    
    log "目录创建完成"
}

# chunk 全量备份
chunk_full_backup() {
    log "开始 chunk 全量备份..."
    
    $PBS_BACKUPER full \
        --chunk-path "$CHUNK_PATH" \
        --remote-path "$CHUNK_REMOTE_PATH" \
        --temp-path "$TEMP_PATH" \
        --rclone-binary "$RCLONE_BINARY" \
        --rclone-config "$RCLONE_CONFIG" \
        --rclone-args "$RCLONE_ARGS" \
        --log-path "$CHUNK_LOG" \
        --prefix-digits "$PREFIX_DIGITS" \
        --timeout "$TIMEOUT" \
        --verbose
    
    local exit_code=$?
    if [ $exit_code -eq 0 ]; then
        log "chunk 全量备份完成"
    else
        log_error "chunk 全量备份失败，退出码: $exit_code"
        return $exit_code
    fi
}

# chunk 增量备份
chunk_incremental_backup() {
    log "开始 chunk 增量备份..."
    
    $PBS_BACKUPER incremental \
        --chunk-path "$CHUNK_PATH" \
        --remote-path "$CHUNK_REMOTE_PATH" \
        --temp-path "$TEMP_PATH" \
        --rclone-binary "$RCLONE_BINARY" \
        --rclone-config "$RCLONE_CONFIG" \
        --rclone-args "$RCLONE_ARGS" \
        --log-path "$CHUNK_LOG" \
        --timeout "$TIMEOUT" \
        --verbose
    
    local exit_code=$?
    if [ $exit_code -eq 0 ]; then
        log "chunk 增量备份完成"
    else
        log_error "chunk 增量备份失败，退出码: $exit_code"
        return $exit_code
    fi
}

# 非 chunk 目录备份（使用 rclone sync）
sync_non_chunk_directories() {
    log "开始同步非 chunk 目录..."
    
    # 设置远程路径，包含日期文件夹
    local remote_target="$REMOTE_BASE/$DATE_FOLDER"
    
    log "源目录: $SOURCE_DIR"
    log "远程目标: $remote_target"
    log "排除目录: .chunks/**"
    
    # 使用 rclone sync 同步，排除 .chunks 目录
    $RCLONE_BINARY sync "$SOURCE_DIR" "$remote_target" \
        $RCLONE_SYNC_ARGS \
        --exclude ".chunks/**" \
        --config "$RCLONE_CONFIG" \
        --log-file "$SYNC_LOG" \
        --log-level INFO
    
    local exit_code=$?
    if [ $exit_code -eq 0 ]; then
        log "非 chunk 目录同步完成"
    else
        log_error "非 chunk 目录同步失败，退出码: $exit_code"
        return $exit_code
    fi
}

# 显示帮助信息
show_help() {
    echo "Comprehensive PVE Backup Server Script"
    echo ""
    echo "用法: $0 [chunk_mode] [options]"
    echo ""
    echo "chunk_mode（必需）:"
    echo "  full         执行 chunk 全量备份"
    echo "  incremental  执行 chunk 增量备份"
    echo ""
    echo "options（可选）:"
    echo "  --chunk-only     仅执行 chunk 备份，跳过其他目录"
    echo "  --sync-only      仅执行其他目录同步，跳过 chunk 备份"
    echo "  --help, -h       显示此帮助信息"
    echo ""
    echo "说明:"
    echo "  - chunk 备份：使用 pbs-backuper 工具进行备份"
    echo "  - 其他目录：使用 rclone sync 按日期创建文件夹备份"
    echo "  - 默认情况下会执行完整的备份流程（chunk + 其他目录）"
    echo ""
    echo "示例:"
    echo "  $0 full                    # 执行完整备份（chunk 全量 + 其他目录）"
    echo "  $0 incremental             # 执行完整备份（chunk 增量 + 其他目录）"
    echo "  $0 full --chunk-only       # 仅执行 chunk 全量备份"
    echo "  $0 incremental --sync-only # 仅执行其他目录同步"
}

# 显示配置信息
show_config() {
    log "=== 备份配置信息 ==="
    log "源目录: $SOURCE_DIR"
    log "chunk 目录: $CHUNK_PATH"
    log "远程基础路径: $REMOTE_BASE"
    log "chunk 远程路径: $CHUNK_REMOTE_PATH"
    log "今日备份路径: $REMOTE_BASE/$DATE_FOLDER"
    log "临时目录: $TEMP_PATH"
    log "日志目录: $LOG_DIR"
    log "主日志文件: $MAIN_LOG"
    log "chunk 日志文件: $CHUNK_LOG"
    log "同步日志文件: $SYNC_LOG"
    log "==================="
}

# ==================== 主程序 ====================

# 检查参数
if [ $# -eq 0 ]; then
    echo "需要指定 chunk 备份模式"
    show_help
    exit 1
fi

# 初始化
create_directories

# 开始日志记录
log "========================================="
log "开始综合备份脚本执行"
log "时间戳: $TIMESTAMP"
log "========================================="

# 显示配置
show_config


# 解析参数
CHUNK_MODE=""
CHUNK_ONLY=false
SYNC_ONLY=false

while [[ $# -gt 0 ]]; do
    case $1 in
        "full"|"incremental")
            if [ -z "$CHUNK_MODE" ]; then
                CHUNK_MODE=$1
            else
                log_error "重复指定 chunk 模式"
                exit 1
            fi
            ;;
        "--chunk-only")
            CHUNK_ONLY=true
            ;;
        "--sync-only")
            SYNC_ONLY=true
            ;;
        "--help"|"-h")
            show_help
            exit 0
            ;;
        *)
            log_error "未知选项: $1"
            show_help
            exit 1
            ;;
    esac
    shift
done

# 验证参数
if [ -z "$CHUNK_MODE" ]; then
    log_error "必须指定 chunk 备份模式 (full/incremental)"
    show_help
    exit 1
fi

if [ "$CHUNK_ONLY" = true ] && [ "$SYNC_ONLY" = true ]; then
    log_error "--chunk-only 和 --sync-only 不能同时使用"
    exit 1
fi

# 检查依赖
check_dependencies

# 执行备份
OVERALL_SUCCESS=true

# 执行 chunk 备份
if [ "$SYNC_ONLY" = false ]; then
    log "开始执行 chunk 备份..."
    
    case "$CHUNK_MODE" in
        "full")
            chunk_full_backup
            if [ $? -ne 0 ]; then
                OVERALL_SUCCESS=false
            fi
            ;;
        "incremental")
            chunk_incremental_backup
            if [ $? -ne 0 ]; then
                OVERALL_SUCCESS=false
            fi
            ;;
    esac
fi

# 执行非 chunk 目录同步
if [ "$CHUNK_ONLY" = false ]; then
    log "开始执行非 chunk 目录同步..."
    sync_non_chunk_directories
    if [ $? -ne 0 ]; then
        OVERALL_SUCCESS=false
    fi
fi

# 总结
log "========================================="
if [ "$OVERALL_SUCCESS" = true ]; then
    log "所有备份任务完成成功！"
    log "主日志文件: $MAIN_LOG"
    if [ "$SYNC_ONLY" = false ]; then
        log "chunk 备份日志: $CHUNK_LOG"
    fi
    if [ "$CHUNK_ONLY" = false ]; then
        log "同步备份日志: $SYNC_LOG"
    fi
    exit 0
else
    log_error "部分或全部备份任务失败，请检查日志文件"
    log_error "主日志文件: $MAIN_LOG"
    if [ "$SYNC_ONLY" = false ]; then
        log_error "chunk 备份日志: $CHUNK_LOG"
    fi
    if [ "$CHUNK_ONLY" = false ]; then
        log_error "同步备份日志: $SYNC_LOG"
    fi
    exit 1
fi