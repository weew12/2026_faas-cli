// Copyright (c) OpenFaaS Author(s) 2023. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命名空间管理命令
// 本文件实现 namespace update 命令：用于更新命名空间的标签和注解
package commands

import (
	"context"
	"fmt"

	"github.com/openfaas/faas-cli/util"
	"github.com/openfaas/faas-provider/types"
	"github.com/spf13/cobra"
)

// NamespaceUpdateFlags 定义更新命名空间的命令行参数结构体
// 存储标签和注解的原始输入参数
type NamespaceUpdateFlags struct {
	labelOpts      []string // 标签参数列表，格式 LABEL=VALUE
	annotationOpts []string // 注解参数列表，格式 ANNOTATION=VALUE
}

// 全局变量，接收命令行参数
var namespaceUpdateFlags NamespaceCreateFlags

// namespaceUpdateCmd 更新命名空间命令
// 用于修改已存在的命名空间的标签(Label)和注解(Annotation)
var namespaceUpdateCmd = &cobra.Command{
	Use: `update NAME
			[--label LABEL=VALUE ...]
			[--annotation ANNOTATION=VALUE ...]`,
	Short: "Update a namespace",
	Long:  "Update a namespace's labels and annotations",
	Example: `  faas-cli namespace update NAME
  faas-cli namespace update NAME --label demo=true
  faas-cli namespace update NAME --annotation demo=true
  faas-cli namespace update NAME --label demo=true \
    --annotation demo=true`,
	RunE:    updateNamespace,    // 命令主执行逻辑
	PreRunE: preUpdateNamespace, // 执行前参数校验
}

// init 初始化命令，注册参数并挂载到 namespace 根命令
func init() {
	// 注册标签参数 -l/--label
	namespaceUpdateCmd.Flags().StringArrayVarP(&namespaceUpdateFlags.labelOpts, "label", "l", []string{}, "Set one or more label (LABEL=VALUE)")
	// 注册注解参数 --annotation
	namespaceUpdateCmd.Flags().StringArrayVarP(&namespaceUpdateFlags.annotationOpts, "annotation", "", []string{}, "Set one or more annotation (ANNOTATION=VALUE)")

	// 将 update 子命令添加到 namespace 根命令
	namespaceCmd.AddCommand(namespaceUpdateCmd)
}

// preUpdateNamespace 命令执行前校验：验证参数数量是否合法
func preUpdateNamespace(cmd *cobra.Command, args []string) error {
	// 必须提供命名空间名称
	if len(args) == 0 {
		return fmt.Errorf("namespace name required")
	}

	// 只能传入一个命名空间名称
	if len(args) > 1 {
		return fmt.Errorf("too many values for namespace name")
	}

	return nil
}

// updateNamespace 执行更新命名空间的核心逻辑
func updateNamespace(cmd *cobra.Command, args []string) error {
	// 获取默认 SDK 客户端（已配置网关、认证、TLS）
	client, err := GetDefaultSDKClient()
	if err != nil {
		return err
	}

	// 解析 --label 参数，将 []string 转为 map[string]string
	labels, err := util.ParseMap(namespaceUpdateFlags.labelOpts, "labels")
	if err != nil {
		return err
	}

	// 解析 --annotation 参数，将 []string 转为 map[string]string
	annotations, err := util.ParseMap(namespaceUpdateFlags.annotationOpts, "annotations")
	if err != nil {
		return err
	}

	// 构造命名空间更新请求体
	req := types.FunctionNamespace{
		Name:        args[0],     // 命名空间名称
		Labels:      labels,      // 新的标签
		Annotations: annotations, // 新的注解
	}

	// 输出更新提示
	fmt.Printf("Updating Namespace: %s\n", req.Name)

	// 调用 SDK 执行更新
	if _, err = client.UpdateNamespace(context.Background(), req); err != nil {
		return err
	}

	// 输出完成提示
	fmt.Printf("Namespace Updated: %s\n", req.Name)

	return nil
}
