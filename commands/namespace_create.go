// Copyright (c) OpenFaaS Author(s) 2023. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命名空间管理命令
// 本文件实现 namespace create 命令：用于创建新的命名空间
package commands

import (
	"context"
	"fmt"

	"github.com/openfaas/faas-cli/util"
	"github.com/openfaas/faas-provider/types"
	"github.com/spf13/cobra"
)

// NamespaceCreateFlags 定义创建命名空间的命令行参数结构体
// 存储创建命名空间时传入的标签和注解原始参数
type NamespaceCreateFlags struct {
	labelOpts      []string // 标签列表，格式 LABEL=VALUE
	annotationOpts []string // 注解列表，格式 ANNOTATION=VALUE
}

// 全局变量，接收命令行输入的参数
var namespaceCreateFlags NamespaceCreateFlags

// namespaceCreateCmd 创建命名空间的子命令
// 支持设置标签和注解，用于创建新的 OpenFaaS 命名空间
var namespaceCreateCmd = &cobra.Command{
	Use: `create NAME
			[--label LABEL=VALUE ...]
			[--annotation ANNOTATION=VALUE ...]`,
	Short: "Create a new namespace",
	Long:  "Create command creates a new namespace",
	Example: `  faas-cli namespace create NAME
  faas-cli namespace create NAME --label demo=true
  faas-cli namespace create NAME --annotation demo=true
  faas-cli namespace create NAME --label demo=true \
    --annotation demo=true`,
	RunE:    createNamespace,    // 命令主执行逻辑
	PreRunE: preCreateNamespace, // 执行前参数校验
}

// init 初始化命令，注册参数并挂载到 namespace 根命令
func init() {
	// 注册 --label / -l 参数
	namespaceCreateCmd.Flags().StringArrayVarP(&namespaceCreateFlags.labelOpts, "label", "l", []string{}, "Set one or more label (LABEL=VALUE)")
	// 注册 --annotation 参数
	namespaceCreateCmd.Flags().StringArrayVarP(&namespaceCreateFlags.annotationOpts, "annotation", "", []string{}, "Set one or more annotation (ANNOTATION=VALUE)")

	// 将 create 子命令添加到命名空间根命令
	namespaceCmd.AddCommand(namespaceCreateCmd)
}

// preCreateNamespace 执行前校验：必须传入且只能传入一个命名空间名称
func preCreateNamespace(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("namespace name required")
	}

	if len(args) > 1 {
		return fmt.Errorf("too many values for namespace name")
	}

	return nil
}

// createNamespace 执行创建命名空间的核心逻辑
func createNamespace(cmd *cobra.Command, args []string) error {
	// 获取默认 SDK 客户端（已配置网关、认证、TLS）
	client, err := GetDefaultSDKClient()
	if err != nil {
		return err
	}

	// 解析 --label 参数，转为 map 格式
	labels, err := util.ParseMap(namespaceCreateFlags.labelOpts, "labels")
	if err != nil {
		return err
	}

	// 解析 --annotation 参数，转为 map 格式
	annotations, err := util.ParseMap(namespaceCreateFlags.annotationOpts, "annotations")
	if err != nil {
		return err
	}

	// 构造创建命名空间的请求体
	req := types.FunctionNamespace{
		Name:        args[0],     // 命名空间名称
		Labels:      labels,      // 标签
		Annotations: annotations, // 注解
	}

	// 输出创建提示
	fmt.Printf("Creating Namespace: %s\n", req.Name)

	// 调用 SDK 接口创建命名空间
	if _, err = client.CreateNamespace(context.Background(), req); err != nil {
		return err
	}

	// 输出创建成功提示
	fmt.Printf("Namespace Created: %s\n", req.Name)

	return nil
}
