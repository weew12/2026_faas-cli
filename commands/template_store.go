// Copyright (c) OpenFaaS Author(s) 2018. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 template store 子命令，用于管理官方模板仓库中的函数模板
package commands

import (
	"github.com/spf13/cobra"
)

// init 初始化函数，将 templateStoreCmd 注册为 templateCmd 的子命令
func init() {
	templateCmd.AddCommand(templateStoreCmd)
}

// templateStoreCmd 模板仓库管理命令
// 用于从官方模板仓库中列出、拉取预定义的函数模板
var templateStoreCmd = &cobra.Command{
	Use:   `store [COMMAND]`,
	Short: `Command for pulling and listing templates from store`,
	Long:  `This command provides the list of the templates from the official store by default`,
	Example: `  faas-cli template store list --verbose
  faas-cli template store ls -v
  faas-cli template store pull ruby-http
  faas-cli template store pull --url=https://raw.githubusercontent.com/openfaas/store/master/templates.json`,
}
