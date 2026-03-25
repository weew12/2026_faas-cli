// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 template pull 命令，用于从 Git 仓库拉取函数模板
package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// 全局命令行参数
var (
	repository string // 模板仓库地址（暂未直接使用，由参数传入）
	overwrite  bool   // 是否覆盖已存在的模板
	pullDebug  bool   // 是否开启调试日志
)

// init 初始化 template pull 命令，注册命令行标志并添加到 template 根命令
func init() {
	templatePullCmd.Flags().BoolVar(&overwrite, "overwrite", true, "Overwrite existing templates?")
	templatePullCmd.Flags().BoolVar(&pullDebug, "debug", false, "Enable debug output")

	templateCmd.AddCommand(templatePullCmd)
}

// templatePullCmd 从指定的 Git 仓库下载函数模板
// 支持指定仓库 URL、分支或标签，自动复制模板目录
var templatePullCmd = &cobra.Command{
	Use:   `pull [REPOSITORY_URL]`,
	Short: `Downloads templates from the specified git repo`,
	Long: `Downloads templates from the specified git repo specified by [REPOSITORY_URL], and copies the 'template'
directory from the root of the repo, if it exists.

[REPOSITORY_URL] may specify a specific branch or tag to copy by adding a URL fragment with the branch or tag name.
	`,
	Example: `
  faas-cli template pull https://github.com/openfaas/templates
  faas-cli template pull https://github.com/openfaas/templates#1.0
`,
	RunE:          runTemplatePull,
	SilenceErrors: true, // 静默错误，由程序自行处理
	SilenceUsage:  true, // 不自动打印使用说明
}

// runTemplatePull 执行 template pull 命令的入口函数
// 解析命令行参数并调用模板拉取逻辑
func runTemplatePull(cmd *cobra.Command, args []string) error {
	repository := ""
	if len(args) > 0 {
		repository = args[0]
	}

	return templatePull(repository, overwrite)
}

// templatePull 处理模板拉取业务逻辑
// 获取最终模板 URL，调用底层方法执行下载
func templatePull(repository string, overwriteTemplates bool) error {
	templateName := ""

	// 获取最终使用的模板仓库地址（命令行参数 > 环境变量 > 默认值）
	repository = getTemplateURL(repository, os.Getenv(templateURLEnvironment), DefaultTemplateRepository)

	return pullTemplate(repository, templateName, overwriteTemplates)
}

// pullDebugPrint 调试打印函数
// 仅在开启 --debug 标志时输出日志
func pullDebugPrint(message string) {
	if pullDebug {
		fmt.Println(message)
	}
}
