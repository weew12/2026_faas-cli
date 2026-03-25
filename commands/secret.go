// Copyright (c) OpenFaaS Author(s) 2018. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 secret 根命令，用于管理函数密钥（secret）
package commands

import (
	"github.com/spf13/cobra"
)

// init 初始化函数，将 secret 根命令注册到主命令 faasCmd
func init() {
	faasCmd.AddCommand(secretCmd)
}

// secretCmd 密钥管理根命令
// 提供子命令用于创建、删除、列出函数密钥
var secretCmd = &cobra.Command{
	Use:   `secret`,
	Short: "OpenFaaS secret commands",
	Long:  "Manage function secrets for secure sensitive data in OpenFaaS functions",
}
