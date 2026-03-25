// Copyright (c) OpenFaaS Author(s) 2023. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// namespaceRemoveCmd 删除命名空间命令
// 别名：rm / delete，用于删除一个已存在的 OpenFaaS 命名空间
var namespaceRemoveCmd = &cobra.Command{
	Use:     `remove NAME`,
	Short:   "Remove existing namespace",
	Long:    "Remove existing namespace",
	Example: `  faas-cli namespace remove NAME`,
	Aliases: []string{"rm", "delete"}, // 支持别名 rm / delete
	RunE:    removeNamespace,          // 主执行函数
	PreRunE: preRemoveNamespace,       // 执行前校验
}

// init 将删除命令注册为 namespace 的子命令
func init() {
	namespaceCmd.AddCommand(namespaceRemoveCmd)
}

// preRemoveNamespace 前置校验：必须传入且只能传入一个命名空间名称
func preRemoveNamespace(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("namespace name required")
	}

	if len(args) > 1 {
		return fmt.Errorf("too many values for namespace name")
	}

	return nil
}

// removeNamespace 执行删除命名空间的核心逻辑
func removeNamespace(cmd *cobra.Command, args []string) error {
	// 获取配置好的 SDK 客户端（网关、token、TLS 已配置）
	client, err := GetDefaultSDKClient()
	if err != nil {
		return err
	}

	// 获取要删除的命名空间名称
	ns := args[0]

	fmt.Printf("Deleting Namespace: %s\n", ns)

	// 调用 SDK 删除命名空间
	if err = client.DeleteNamespace(context.Background(), ns); err != nil {
		return err
	}

	fmt.Printf("Namespace Removed: %s\n", ns)

	return nil
}
