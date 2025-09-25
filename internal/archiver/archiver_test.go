package archiver

import (
	"os"
	"path/filepath"
	"testing"

	"pbs-backuper/internal/models"
)

// TestArchiveGrouping 测试压缩包分组逻辑
func TestArchiveGrouping(t *testing.T) {
	testCases := []struct {
		name           string
		prefixDigits   int
		directories    []string
		expectedGroups map[string][]string // prefix -> directories
	}{
		{
			name:         "2位前缀分组",
			prefixDigits: 2,
			directories:  []string{"0000", "0001", "00ff", "0100", "01aa", "abcd", "ffff"},
			expectedGroups: map[string][]string{
				"00": {"0000", "0001", "00ff"},
				"01": {"0100", "01aa"},
				"ab": {"abcd"},
				"ff": {"ffff"},
			},
		},
		{
			name:         "1位前缀分组",
			prefixDigits: 1,
			directories:  []string{"0000", "0123", "1abc", "abcd", "ffff"},
			expectedGroups: map[string][]string{
				"0": {"0000", "0123"},
				"1": {"1abc"},
				"a": {"abcd"},
				"f": {"ffff"},
			},
		},
		{
			name:         "3位前缀分组",
			prefixDigits: 3,
			directories:  []string{"0000", "0001", "000a", "001b", "abcd"},
			expectedGroups: map[string][]string{
				"000": {"0000", "0001", "000a"},
				"001": {"001b"},
				"abc": {"abcd"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建临时目录
			tempDir := t.TempDir()
			archiver := NewArchiver(tempDir, tempDir)

			// 生成压缩包分组
			groups, err := archiver.GenerateArchiveGroups(tc.directories, tc.prefixDigits)
			if err != nil {
				t.Fatalf("生成压缩包分组失败: %v", err)
			}

			// 验证分组数量
			if len(groups) != len(tc.expectedGroups) {
				t.Errorf("期望 %d 个分组，实际得到 %d 个", len(tc.expectedGroups), len(groups))
			}

			// 验证每个分组
			groupMap := make(map[string]*models.ArchiveGroup)
			for _, group := range groups {
				groupMap[group.Prefix] = group
			}

			for expectedPrefix, expectedDirs := range tc.expectedGroups {
				group, exists := groupMap[expectedPrefix]
				if !exists {
					t.Errorf("缺少前缀 %s 的分组", expectedPrefix)
					continue
				}

				// 验证目录列表
				if len(group.Directories) != len(expectedDirs) {
					t.Errorf("前缀 %s: 期望 %d 个目录，实际 %d 个",
						expectedPrefix, len(expectedDirs), len(group.Directories))
					continue
				}

				for _, expectedDir := range expectedDirs {
					found := false
					for _, actualDir := range group.Directories {
						if actualDir == expectedDir {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("前缀 %s 的分组中缺少目录 %s", expectedPrefix, expectedDir)
					}
				}

				// 验证压缩包命名
				expectedArchiveName := group.StartRange + "-" + group.EndRange + ".tar.gz"
				if group.ArchiveName != expectedArchiveName {
					t.Errorf("前缀 %s: 期望压缩包名 %s，实际 %s",
						expectedPrefix, expectedArchiveName, group.ArchiveName)
				}

				t.Logf("前缀 %s: %s (%d个目录)",
					group.Prefix, group.ArchiveName, len(group.Directories))
			}
		})
	}
}

// TestArchiveCreation 测试压缩包创建
func TestArchiveCreation(t *testing.T) {
	// 创建测试环境
	testDir := t.TempDir()
	chunkDir := filepath.Join(testDir, "chunks")
	tempDir := filepath.Join(testDir, "temp")

	// 创建chunk目录和文件
	testDirs := []string{"0000", "0001"}
	for _, dir := range testDirs {
		dirPath := filepath.Join(chunkDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("创建目录失败: %v", err)
		}

		// 创建测试文件
		for i := 0; i < 2; i++ {
			fileName := filepath.Join(dirPath, "file"+string(rune('0'+i))+".txt")
			content := "测试内容 " + dir + " file " + string(rune('0'+i))
			if err := os.WriteFile(fileName, []byte(content), 0644); err != nil {
				t.Fatalf("创建文件失败: %v", err)
			}
		}

		// 创建子目录
		subDir := filepath.Join(dirPath, "subdir")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("创建子目录失败: %v", err)
		}
		subFile := filepath.Join(subDir, "subfile.txt")
		if err := os.WriteFile(subFile, []byte("子目录文件"), 0644); err != nil {
			t.Fatalf("创建子文件失败: %v", err)
		}
	}

	// 创建archiver
	archiver := NewArchiver(chunkDir, tempDir)

	// 生成分组
	groups, err := archiver.GenerateArchiveGroups(testDirs, 2)
	if err != nil {
		t.Fatalf("生成分组失败: %v", err)
	}

	if len(groups) != 1 {
		t.Fatalf("应该生成1个分组，实际生成 %d 个", len(groups))
	}

	group := groups[0]

	// 创建压缩包
	archivePath, err := archiver.CreateArchive(group)
	if err != nil {
		t.Fatalf("创建压缩包失败: %v", err)
	}

	// 验证压缩包文件存在
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Error("压缩包文件不存在")
	}

	// 计算校验和
	checksum, err := archiver.CalculateChecksum(archivePath)
	if err != nil {
		t.Fatalf("计算校验和失败: %v", err)
	}

	if len(checksum) != 64 { // SHA256是64个字符
		t.Errorf("校验和长度不正确: %d", len(checksum))
	}

	// 创建校验和文件
	checksumPath, err := archiver.CreateChecksumFile(archivePath, checksum)
	if err != nil {
		t.Fatalf("创建校验和文件失败: %v", err)
	}

	// 验证校验和文件存在
	if _, err := os.Stat(checksumPath); os.IsNotExist(err) {
		t.Error("校验和文件不存在")
	}

	t.Logf("成功创建压缩包: %s", archivePath)
	t.Logf("校验和: %s", checksum)
	t.Logf("校验和文件: %s", checksumPath)
}

// TestMarkGroupsForUpdate 测试标记需要更新的分组
func TestMarkGroupsForUpdate(t *testing.T) {
	tempDir := t.TempDir()
	archiver := NewArchiver(tempDir, tempDir)

	// 创建测试分组
	directories := []string{"0000", "0001", "0100", "0101"}
	groups, err := archiver.GenerateArchiveGroups(directories, 2)
	if err != nil {
		t.Fatalf("生成分组失败: %v", err)
	}

	// 模拟变化的目录
	changedDirs := map[string]bool{
		"0000": true, // 00前缀组应该被标记
		"0101": true, // 01前缀组应该被标记
	}

	// 标记需要更新的分组
	archiver.MarkGroupsForUpdate(groups, changedDirs)

	// 验证标记结果
	for _, group := range groups {
		switch group.Prefix {
		case "00":
			if !group.NeedsUpdate {
				t.Error("00前缀组应该被标记为需要更新")
			}
		case "01":
			if !group.NeedsUpdate {
				t.Error("01前缀组应该被标记为需要更新")
			}
		default:
			if group.NeedsUpdate {
				t.Errorf("前缀 %s 的组不应该被标记为需要更新", group.Prefix)
			}
		}
	}

	t.Log("分组更新标记测试通过")
}
