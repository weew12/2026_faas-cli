// Copyright (c) OpenFaaS Author(s) 2025. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// init 初始化 plugin 命令，将其注册到主命令 faasCmd
func init() {
	faasCmd.AddCommand(pluginCmd)
}

// pluginCmd 插件管理的根命令，用于管理 OpenFaaS CLI 插件
// 该命令仅作为入口，实际功能由子命令（如 plugin get）实现
var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage plugins",
	Long:  `Manage plugins for the OpenFaaS CLI`,
	RunE:  runPlugin,
}

// runPlugin 插件根命令的执行逻辑
// 仅提示用户使用子命令（如 plugin get --help），本身不执行具体操作
func runPlugin(cmd *cobra.Command, args []string) error {
	fmt.Println("Run plugin get --help")
	return nil
}
