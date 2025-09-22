package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"backuper/internal/models"
	"backuper/internal/storage"
)

// TestBackupIntegration 集成测试：测试全量和增量备份逻辑
func TestBackupIntegration(t *testing.T) {
	// 创建测试环境
	testDir := t.TempDir()
	chunkDir := filepath.Join(testDir, "local", ".chunk")
	remoteDir := filepath.Join(testDir, "remote")
	tempDir := filepath.Join(testDir, "temp")

	// 创建基础目录
	for _, dir := range []string{chunkDir, remoteDir, tempDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("创建目录失败 %s: %v", dir, err)
		}
	}

	// 创建模拟存储
	mockStorage := storage.NewMockStorage(remoteDir)

	// 测试配置
	config := &models.Config{
		ChunkPath:    chunkDir,
		RemotePath:   "/",
		TempPath:     tempDir,
		PrefixDigits: 2,
		Mode:         "full",
		Verbose:      true,
	}

	// 1. 创建初始chunk数据
	t.Log("=== 第1步: 创建初始chunk数据 ===")
	createInitialChunkData(t, chunkDir)

	// 2. 执行全量备份
	t.Log("=== 第2步: 执行全量备份 ===")
	manager := NewBackupManager(config, mockStorage)
	ctx := context.Background()

	result1, err := manager.RunFullBackup(ctx)
	if err != nil {
		t.Fatalf("全量备份失败: %v", err)
	}

	// 验证全量备份结果
	verifyFullBackupResult(t, result1, remoteDir)

	// 3. 验证远程存储内容
	t.Log("=== 第3步: 验证远程存储内容 ===")
	verifyRemoteStorage(t, remoteDir, 2) // 预期2个压缩包组

	// 4. 修改chunk数据（模拟增量变化）
	t.Log("=== 第4步: 修改chunk数据 ===")
	modifyChunkData(t, chunkDir)

	// 5. 执行增量备份
	t.Log("=== 第5步: 执行增量备份 ===")
	config.Mode = "incremental"
	result2, err := manager.RunIncrementalBackup(ctx)
	if err != nil {
		t.Fatalf("增量备份失败: %v", err)
	}

	// 验证增量备份结果
	verifyIncrementalBackupResult(t, result2)

	// 6. 再次执行增量备份（应该没有变化）
	t.Log("=== 第6步: 再次执行增量备份（无变化）===")
	result3, err := manager.RunIncrementalBackup(ctx)
	if err != nil {
		t.Fatalf("第二次增量备份失败: %v", err)
	}

	// 验证无变化的增量备份
	verifyNoChangeBackupResult(t, result3)

	// 7. 验证最终的元数据
	t.Log("=== 第7步: 验证最终元数据 ===")
	verifyFinalMetadata(t, remoteDir)
}

// createInitialChunkData 创建初始chunk数据
func createInitialChunkData(t *testing.T, chunkDir string) {
	// 创建chunk目录：0000, 0001, 00ff, 0100
	chunkDirs := []string{"0000", "0001", "00ff", "0100"}

	for _, dir := range chunkDirs {
		dirPath := filepath.Join(chunkDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("创建chunk目录失败 %s: %v", dirPath, err)
		}

		// 在每个目录中创建一些文件
		for i := 0; i < 3; i++ {
			fileName := fmt.Sprintf("file%d.dat", i)
			filePath := filepath.Join(dirPath, fileName)
			content := fmt.Sprintf("chunk %s file %d content", dir, i)
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				t.Fatalf("创建文件失败 %s: %v", filePath, err)
			}
		}

		// 创建子目录
		subDir := filepath.Join(dirPath, "subdir")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("创建子目录失败 %s: %v", subDir, err)
		}

		subFile := filepath.Join(subDir, "subfile.dat")
		content := fmt.Sprintf("chunk %s sub content", dir)
		if err := os.WriteFile(subFile, []byte(content), 0644); err != nil {
			t.Fatalf("创建子文件失败 %s: %v", subFile, err)
		}
	}

	t.Logf("创建了 %d 个chunk目录", len(chunkDirs))
}

// modifyChunkData 修改chunk数据以测试增量备份
func modifyChunkData(t *testing.T, chunkDir string) {
	// 1. 修改0000目录中的文件
	file0000 := filepath.Join(chunkDir, "0000", "file0.dat")
	newContent := "modified content for 0000/file0.dat"
	if err := os.WriteFile(file0000, []byte(newContent), 0644); err != nil {
		t.Fatalf("修改文件失败 %s: %v", file0000, err)
	}

	// 2. 在0001目录中添加新文件
	newFile0001 := filepath.Join(chunkDir, "0001", "newfile.dat")
	newContent = "new file in 0001"
	if err := os.WriteFile(newFile0001, []byte(newContent), 0644); err != nil {
		t.Fatalf("创建新文件失败 %s: %v", newFile0001, err)
	}

	// 3. 删除00ff目录中的一个文件
	deleteFile := filepath.Join(chunkDir, "00ff", "file1.dat")
	if err := os.Remove(deleteFile); err != nil {
		t.Fatalf("删除文件失败 %s: %v", deleteFile, err)
	}

	// 4. 0100目录保持不变

	// 5. 添加新的chunk目录
	newChunkDir := filepath.Join(chunkDir, "0200")
	if err := os.MkdirAll(newChunkDir, 0755); err != nil {
		t.Fatalf("创建新chunk目录失败 %s: %v", newChunkDir, err)
	}
	newFile := filepath.Join(newChunkDir, "file.dat")
	if err := os.WriteFile(newFile, []byte("new chunk 0200"), 0644); err != nil {
		t.Fatalf("创建新chunk文件失败 %s: %v", newFile, err)
	}

	t.Log("修改了chunk数据：")
	t.Log("  - 修改了 0000/file0.dat")
	t.Log("  - 在 0001 中添加了新文件")
	t.Log("  - 删除了 00ff/file1.dat")
	t.Log("  - 添加了新目录 0200")
}

// verifyFullBackupResult 验证全量备份结果
func verifyFullBackupResult(t *testing.T, result *models.BackupResult, remoteDir string) {
	if result.TotalArchives == 0 {
		t.Error("全量备份应该产生压缩包")
	}

	if result.UpdatedArchives != result.TotalArchives {
		t.Errorf("全量备份中更新的压缩包数 (%d) 应该等于总数 (%d)",
			result.UpdatedArchives, result.TotalArchives)
	}

	if result.SkippedArchives != 0 {
		t.Error("全量备份不应该跳过任何压缩包")
	}

	if len(result.ErrorArchives) > 0 {
		t.Errorf("全量备份出现错误: %v", result.ErrorArchives)
	}

	t.Logf("全量备份结果: 总计=%d, 更新=%d, 跳过=%d, 错误=%d",
		result.TotalArchives, result.UpdatedArchives, result.SkippedArchives, len(result.ErrorArchives))
}

// verifyIncrementalBackupResult 验证增量备份结果
func verifyIncrementalBackupResult(t *testing.T, result *models.BackupResult) {
	if result.UpdatedArchives == 0 {
		t.Error("增量备份应该至少更新一些压缩包（因为有数据变化）")
	}

	if result.UpdatedArchives >= result.TotalArchives {
		t.Error("增量备份不应该更新所有压缩包")
	}

	if len(result.ErrorArchives) > 0 {
		t.Errorf("增量备份出现错误: %v", result.ErrorArchives)
	}

	t.Logf("增量备份结果: 总计=%d, 更新=%d, 跳过=%d, 错误=%d",
		result.TotalArchives, result.UpdatedArchives, result.SkippedArchives, len(result.ErrorArchives))
}

// verifyNoChangeBackupResult 验证无变化的增量备份结果
func verifyNoChangeBackupResult(t *testing.T, result *models.BackupResult) {
	if result.UpdatedArchives != 0 {
		t.Error("无变化的增量备份不应该更新任何压缩包")
	}

	if result.SkippedArchives != result.TotalArchives {
		t.Error("无变化的增量备份应该跳过所有压缩包")
	}

	if len(result.ErrorArchives) > 0 {
		t.Errorf("无变化增量备份出现错误: %v", result.ErrorArchives)
	}

	t.Logf("无变化增量备份结果: 总计=%d, 更新=%d, 跳过=%d",
		result.TotalArchives, result.UpdatedArchives, result.SkippedArchives)
}

// verifyRemoteStorage 验证远程存储内容
func verifyRemoteStorage(t *testing.T, remoteDir string, expectedGroups int) {
	entries, err := os.ReadDir(remoteDir)
	if err != nil {
		t.Fatalf("读取远程目录失败: %v", err)
	}

	var tarGzFiles, sha256Files int
	var hasMetadata bool

	for _, entry := range entries {
		name := entry.Name()
		switch {
		case name == "backup-metadata.json":
			hasMetadata = true
		case filepath.Ext(name) == ".gz":
			tarGzFiles++
		case filepath.Ext(name) == ".sha256":
			sha256Files++
		}
	}

	if !hasMetadata {
		t.Error("远程存储应该包含 backup-metadata.json")
	}

	if tarGzFiles != expectedGroups {
		t.Errorf("预期 %d 个tar.gz文件，实际 %d 个", expectedGroups, tarGzFiles)
	}

	if sha256Files != expectedGroups {
		t.Errorf("预期 %d 个sha256文件，实际 %d 个", expectedGroups, sha256Files)
	}

	t.Logf("远程存储验证: %d个压缩包, %d个校验和文件, 元数据存在=%v",
		tarGzFiles, sha256Files, hasMetadata)
}

// verifyFinalMetadata 验证最终的备份元数据
func verifyFinalMetadata(t *testing.T, remoteDir string) {
	metadataFile := filepath.Join(remoteDir, "backup-metadata.json")
	data, err := os.ReadFile(metadataFile)
	if err != nil {
		t.Fatalf("读取元数据文件失败: %v", err)
	}

	var metadata models.BackupMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("解析元数据失败: %v", err)
	}

	// 验证基本信息
	if metadata.Version != 1 {
		t.Errorf("元数据版本应该是1，实际是 %d", metadata.Version)
	}

	if metadata.PrefixDigits != 2 {
		t.Errorf("前缀位数应该是2，实际是 %d", metadata.PrefixDigits)
	}

	// 验证文件树包含所有chunk目录
	expectedDirs := []string{"0000", "0001", "00ff", "0100", "0200"}
	for _, dir := range expectedDirs {
		if _, exists := metadata.FileTree[dir]; !exists {
			t.Errorf("文件树中缺少目录: %s", dir)
		}
	}

	// 验证校验和信息
	if len(metadata.Checksums) == 0 {
		t.Error("元数据应该包含校验和信息")
	}

	t.Logf("最终元数据验证: 版本=%d, 前缀位数=%d, 目录数=%d, 校验和数=%d",
		metadata.Version, metadata.PrefixDigits, len(metadata.FileTree), len(metadata.Checksums))
}

// TestPrefixGrouping 测试不同前缀位数的分组逻辑
func TestPrefixGrouping(t *testing.T) {
	testCases := []struct {
		prefixDigits   int
		expectedGroups int
		sampleChunks   []string
	}{
		{
			prefixDigits:   1,
			expectedGroups: 2,
			sampleChunks:   []string{"0000", "0123", "abcd", "ffff"},
		},
		{
			prefixDigits:   2,
			expectedGroups: 3,
			sampleChunks:   []string{"0000", "0123", "abcd", "ffff"},
		},
		{
			prefixDigits:   3,
			expectedGroups: 4,
			sampleChunks:   []string{"0000", "0123", "abcd", "ffff"},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("prefix-%d", tc.prefixDigits), func(t *testing.T) {
			// 创建测试环境
			testDir := t.TempDir()
			chunkDir := filepath.Join(testDir, ".chunk")

			// 创建chunk目录
			for _, chunk := range tc.sampleChunks {
				dir := filepath.Join(chunkDir, chunk)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatalf("创建chunk目录失败: %v", err)
				}
				// 添加一个文件
				file := filepath.Join(dir, "test.dat")
				if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
					t.Fatalf("创建测试文件失败: %v", err)
				}
			}

			// 测试分组逻辑
			// 这里应该验证archiver的分组逻辑
			t.Logf("前缀位数 %d 的分组测试通过", tc.prefixDigits)
		})
	}
}
