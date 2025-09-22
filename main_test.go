package main

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// 运行所有测试
	code := m.Run()

	// 退出
	os.Exit(code)
}
