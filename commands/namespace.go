// Copyright (c) OpenFaaS Author(s) 2023. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命名空间管理的根命令
// 本文件定义 namespace 命令入口，所有命名空间子命令（list/create/delete等）都挂载在此命令下
package commands

import (
	"github.com/spf13/cobra"
)

// init 初始化命名空间根命令，注册全局持久化参数并添加到主命令
func init() {
	// 设置所有子命令共用的全局参数（定义在 faas.go 中）
	// PersistentFlags() 表示参数对所有子命令生效
	namespaceCmd.PersistentFlags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL starting with http(s)://")
	namespaceCmd.PersistentFlags().BoolVar(&tlsInsecure, "tls-no-verify", false, "Disable TLS validation")
	namespaceCmd.PersistentFlags().StringVarP(&token, "token", "k", "", "Pass a JWT token to use instead of basic auth")

	// 将命名空间根命令注册到 faas-cli 主命令
	faasCmd.AddCommand(namespaceCmd)
}

// namespaceCmd 命名空间管理的**根命令**，不执行业务逻辑，仅作为子命令的入口
// 支持别名 ns，可用于管理 OpenFaaS 命名空间（查询、创建、更新、删除）
var namespaceCmd = &cobra.Command{
	Use:     `namespace [--gateway GATEWAY_URL] [--tls-no-verify] [--token JWT_TOKEN]`,
	Aliases: []string{"ns"}, // 命令别名：faas-cli ns 等价于 faas-cli namespace
	Short:   "Manage OpenFaaS namespaces",
	Long:    "Query, create, update, and delete OpenFaaS namespaces",
}
