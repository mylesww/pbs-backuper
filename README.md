# Backuper

PVE备份服务器chunk数据备份工具，支持增量备份和rclone云存储。

## 功能特性

- **全量备份**: 创建所有chunk目录的压缩包，支持可配置的前缀分组
- **增量备份**: 基于文件树比较，仅处理变化的目录
- **SHA256校验**: 为所有压缩包生成和验证校验和
- **云存储支持**: 通过rclone支持任何云存储提供商
- **结构化元数据**: 以JSON格式维护备份元数据用于变更跟踪
- **可配置分组**: 按十六进制前缀分组chunk目录（1-4位）

## 安装

### 前置要求

- Go 1.19 或更高版本
- rclone（已配置云存储）

### 从源码构建

```bash
git clone <仓库地址>
cd backuper
go mod download
go build -o backuper .
```

## 使用方法

### 全量备份

执行所有chunk目录的完整备份：

```bash
./backuper full --chunk-path /path/to/.chunk --remote-path remote:backup --prefix-digits 2
```

### 增量备份

执行增量备份（需要先有全量备份）：

```bash
./backuper incremental --chunk-path /path/to/.chunk --remote-path remote:backup
```

### 命令行选项

#### 全局选项

- `--chunk-path`: .chunk目录路径（必需）
- `--remote-path`: 远程存储路径（必需）
- `--temp-path`: 临时文件路径（默认: /tmp/backuper）
- `--rclone-binary`: rclone二进制文件路径（默认: rclone）
- `--rclone-config`: rclone配置文件路径
- `--rclone-args`: 额外的rclone参数（逗号分隔）
- `--verbose, -v`: 启用详细输出
- `--timeout`: 操作超时时间（默认: 30m）
- `--log-path`: 日志文件路径（可选，默认仅输出到控制台）

#### 全量备份选项

- `--prefix-digits`: 分组前缀位数（1-4，默认: 2）

## 工作原理

### 目录分组

工具按十六进制前缀对chunk目录进行分组：

- **前缀位数 = 2**: `0000-00ff.tar.gz`, `0100-01ff.tar.gz`, 等等
- **前缀位数 = 3**: `0000-000f.tar.gz`, `0010-001f.tar.gz`, 等等

### 增量备份逻辑

1. 从远程存储下载之前的备份元数据
2. 扫描当前chunk目录结构
3. 比较文件树以识别变化：
   - 新文件/目录
   - 修改的文件（大小或时间戳变化）
   - 删除的文件/目录
4. 仅重新创建包含变化的组的压缩包
5. 上传前验证校验和
6. 用当前状态更新元数据

### 文件结构

```
远程存储:
├── 0000-00ff.tar.gz       # 目录0000-00ff的压缩包
├── 0000-00ff.tar.gz.sha256 # SHA256校验和
├── 0100-01ff.tar.gz       # 目录0100-01ff的压缩包
├── 0100-01ff.tar.gz.sha256 # SHA256校验和
├── ...
└── backup-metadata.json   # 备份元数据和文件树
```

## 使用示例

### 基本用法

```bash
# 使用2位前缀分组的全量备份
./backuper full \
  --chunk-path /var/lib/vz/backup/.chunks \
  --remote-path s3:my-bucket/pve-backups \
  --prefix-digits 2

# 增量备份
./backuper incremental \
  --chunk-path /var/lib/vz/backup/.chunks \
  --remote-path s3:my-bucket/pve-backups
```

### 高级配置

```bash
# 使用自定义rclone设置和详细日志
./backuper full \
  --chunk-path /var/lib/vz/backup/.chunks \
  --remote-path backup-remote:pve/chunks \
  --rclone-binary /usr/local/bin/rclone \
  --rclone-config /root/.config/rclone/rclone.conf \
  --rclone-args "--transfers=8,--checkers=16,--progress" \
  --prefix-digits 3 \
  --verbose \
  --log-path /var/log/backuper.log \
  --timeout 2h
```

### Cron自动化

```bash
# 每日凌晨2点增量备份
0 2 * * * /usr/local/bin/backuper incremental --chunk-path /var/lib/vz/backup/.chunks --remote-path s3:backup/pve

# 每周日凌晨1点全量备份
0 1 * * 0 /usr/local/bin/backuper full --chunk-path /var/lib/vz/backup/.chunks --remote-path s3:backup/pve --prefix-digits 2
```

## 配置

### Rclone设置

使用云存储提供商配置rclone：

```bash
rclone config
```

测试配置：

```bash
rclone ls remote:bucket/path
```

### 环境变量

也可以使用环境变量进行某些设置：

```bash
export RCLONE_CONFIG=/path/to/rclone.conf
export BACKUPER_TEMP_PATH=/tmp/backuper
```

## 监控和日志

### 日志级别

- **Info**: 基本操作进度
- **Debug**: 详细操作信息（使用`--verbose`启用）
- **Warn**: 非致命问题
- **Error**: 操作失败

### 日志输出

```bash
# 记录到文件和控制台
./backuper full --chunk-path /path/to/.chunks --remote-path remote:backup --log-path /var/log/backuper.log

# 仅控制台（默认）
./backuper full --chunk-path /path/to/.chunks --remote-path remote:backup
```

## 故障排除

### 常见问题

1. **权限拒绝**: 确保用户对chunk目录有读权限，对临时路径有写权限
2. **Rclone错误**: 验证rclone配置和网络连接
3. **磁盘空间**: 确保临时目录有足够空间存储压缩包
4. **超时问题**: 对于大数据集增加超时时间

### 调试模式

启用详细日志来排除问题：

```bash
./backuper full --chunk-path /path/to/.chunks --remote-path remote:backup --verbose
```

### 恢复

从失败的备份中恢复：

1. 检查日志中的具体错误信息
2. 验证rclone配置和连接性
3. 确保有足够的磁盘空间和权限
4. 重新运行备份（增量备份可以安全重试）

## 架构

### 组件

- **Scanner**: 扫描chunk目录并构建文件树
- **Archiver**: 创建tar.gz压缩包并计算校验和
- **Storage**: 云存储操作的抽象接口
- **Backup Manager**: 协调备份过程
- **CLI**: 使用Cobra的命令行界面

### 存储接口

存储接口可以扩展以支持其他云提供商：

```go
type Storage interface {
    ListFiles(ctx context.Context, remotePath string) ([]FileInfo, error)
    DownloadFile(ctx context.Context, remotePath, localPath string) error
    UploadFile(ctx context.Context, localPath, remotePath string) error
    FileExists(ctx context.Context, remotePath string) (bool, error)
    GetFileContent(ctx context.Context, remotePath string) ([]byte, error)
}
```

## 开发

### 运行测试

```bash
go test ./...
```

### 构建

```bash
go build -o backuper .
```

### 贡献

1. Fork仓库
2. 创建功能分支
3. 为新功能添加测试
4. 确保所有测试通过
5. 提交Pull Request

## 许可证

[在此添加您的许可证]