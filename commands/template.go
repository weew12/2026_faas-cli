// Copyright (c) OpenFaaS Author(s) 2018. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 template 根命令，用于管理函数模板（拉取、查看、搜索模板）
package commands

import (
	"github.com/spf13/cobra"
)

// init 初始化函数，将 template 根命令注册到 CLI 主命令 faasCmd
func init() {
	faasCmd.AddCommand(templateCmd)
}

// templateCmd 模板管理根命令，提供子命令用于操作 OpenFaaS 函数模板
// 包含：从官方模板仓库查看/拉取模板、拉取自定义远程模板等功能
var templateCmd = &cobra.Command{
	Use:   `template [COMMAND]`,
	Short: "OpenFaaS template store and pull commands",
	Long:  "Allows browsing templates from store or pulling custom templates",
	Example: `  faas-cli template pull https://github.com/custom/template
  faas-cli template store list
  faas-cli template store ls
  faas-cli template store pull ruby-http
  faas-cli template store pull openfaas-incubator/ruby-http`,
}
