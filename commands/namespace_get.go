// Copyright (c) OpenFaaS Author(s) 2023. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/openfaas/faas-provider/types"
	"github.com/spf13/cobra"
)

// namespaceGetCmd 查询单个命名空间详情的子命令
// 用于获取指定命名空间的名称、标签、注解等详细信息
var namespaceGetCmd = &cobra.Command{
	Use:     `get NAME`,
	Short:   "Get existing namespace",
	Long:    "Get existing namespace",
	Example: `  faas-cli namespace get NAME`,
	RunE:    get_namespace,   // 命令执行入口
	PreRunE: preGetNamespace, // 执行前参数校验
}

// init 初始化命令，将 get 子命令挂载到 namespace 根命令
func init() {
	namespaceCmd.AddCommand(namespaceGetCmd)
}

// preGetNamespace 前置校验：必须传入且只能传入一个命名空间名称
func preGetNamespace(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("namespace name required")
	}

	if len(args) > 1 {
		return fmt.Errorf("too many values for namespace name")
	}

	return nil
}

// get_namespace 执行查询命名空间的核心逻辑
func get_namespace(cmd *cobra.Command, args []string) error {
	// 获取默认 SDK 客户端（已配置网关、认证、TLS）
	client, err := GetDefaultSDKClient()
	if err != nil {
		return err
	}

	// 获取要查询的命名空间名称
	ns := args[0]

	// 调用 SDK 接口查询命名空间详情
	res, err := client.GetNamespace(context.Background(), ns)
	if err != nil {
		return err
	}

	// 格式化打印命名空间详情
	printNamespaceDetail(cmd.OutOrStdout(), res)

	return nil
}

// printNamespaceDetail 格式化打印命名空间详细信息
// 使用 tabwriter 实现对齐输出，提升可读性
func printNamespaceDetail(dst io.Writer, nsDetail types.FunctionNamespace) {
	w := tabwriter.NewWriter(dst, 0, 0, 1, ' ', tabwriter.TabIndent)
	defer w.Flush()

	out := printer{
		w:       w,
		verbose: verbose,
	}

	// 打印命名空间名称
	out.Printf("Name:\t%s\n", nsDetail.Name)

	// 打印标签 Labels
	if len(nsDetail.Labels) > 0 {
		out.Printf("Labels:\t%v\n", nsDetail.Labels)
	} else {
		out.Printf("Labels:\t%v\n", map[string]string{})
	}

	// 打印注解 Annotations
	if len(nsDetail.Annotations) > 0 {
		out.Printf("Annotations:\t%v\n", nsDetail.Annotations)
	} else {
		out.Printf("Annotations:\t%v\n", map[string]string{})
	}
}
