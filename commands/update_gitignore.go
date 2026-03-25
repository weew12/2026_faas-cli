// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 .gitignore 文件自动更新功能，用于添加默认忽略目录
package commands

import (
	"os"
	"strings"
)

// contains 判断字符串切片中是否包含指定字符串
// 参数：
//
//	s - 字符串切片
//	e - 要查找的目标字符串
//
// 返回：
//
//	bool - 存在返回 true，不存在返回 false
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// updateContent 更新文件内容，添加默认需要忽略的目录
// 参数：
//
//	content - 原始文件内容字符串
//
// 返回：
//
//	updated_content - 添加忽略目录后的完整文件内容
func updateContent(content string) (updated_content string) {
	// 需要添加到 .gitignore 的默认目录列表
	filesToIgnore := []string{"template", "build", ".secrets"}

	// 将原始内容按行拆分
	lines := strings.Split(content, "\n")

	// 遍历需要忽略的目录，不存在则追加
	for _, file := range filesToIgnore {
		if !contains(lines, file) {
			lines = append(lines, file)
		}
	}

	// 重新拼接为字符串并去除首尾多余换行符
	updated_content = strings.Join(lines, "\n")
	updated_content = strings.Trim(updated_content, "\n")
	return updated_content
}

// updateGitignore 创建或更新 .gitignore 文件
// 若文件不存在则创建，存在则追加默认忽略目录（template/build/.secrets）
// 返回：
//
//	err - 执行过程中发生的错误
func updateGitignore() (err error) {
	// 以读写模式打开 .gitignore，不存在则创建
	f, err := os.OpenFile(".gitignore", os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	// 函数结束时关闭文件句柄
	defer f.Close()

	// 读取当前 .gitignore 全部内容
	content, err := os.ReadFile(".gitignore")
	if err != nil {
		return err
	}

	// 转换为字符串并执行更新逻辑
	string_content := string(content[:])
	write_content := updateContent(string_content)

	// 将更新后的内容写回文件，末尾补充换行保证格式规范
	_, err = f.WriteString(write_content + "\n")
	if err != nil {
		return err
	}

	return nil
}
