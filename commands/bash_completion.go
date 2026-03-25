// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 CLI 命令行工具的所有命令逻辑
// 本文件提供 bash 自动补全文件生成功能
package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// init 初始化函数，将 bashcompletion 命令注册到根命令 faasCmd
func init() {
	faasCmd.AddCommand(bashcompletionCmd)
}

// bashcompletionCmd 定义 bashcompletion 子命令，用于生成 Bash 自动补全脚本文件
// 已废弃，建议使用 completion 命令替代
// 该命令默认隐藏，仅支持 Bash 4+ 版本
var bashcompletionCmd = &cobra.Command{
	Use:   "bashcompletion FILENAME",
	Short: "Generate a bash completion file",
	Long: `Generate a bash completion file for the client.

This currently only works on Bash version 4, and is hidden
pending a merge of https://github.com/spf13/cobra/pull/520.`,
	Hidden:     true,
	Deprecated: `please use the "completion" command`,
	RunE:       runBashcompletion,
}

// runBashcompletion 执行 bashcompletion 命令的核心逻辑
// 接收命令行参数，校验文件名并生成 bash 补全文件
// 参数：cmd - cobra 命令对象；args - 命令行参数
// 返回值：执行过程中发生的错误
func runBashcompletion(cmd *cobra.Command, args []string) error {
	// 校验是否传入文件名参数
	if len(args) < 1 {
		return fmt.Errorf("please provide filename for bash completion")
	}
	fileName := args[0]

	// 调用 cobra 内置方法生成 bash 补全文件
	err := faasCmd.GenBashCompletionFile(fileName)
	if err != nil {
		return fmt.Errorf("unable to create bash completion file")
	}

	return nil
}
