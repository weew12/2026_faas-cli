// Package commands 实现 OpenFaaS CLI 命名空间管理命令
// 本文件实现：列出 OpenFaaS 服务端支持的命名空间（namespace list）
package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// init 初始化命名空间相关命令，注册标志并添加到主命令
func init() {
	// 共用标志：网关地址、TLS 验证、JWT 认证令牌
	namespacesCmd.Flags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL starting with http(s)://")
	namespacesCmd.Flags().BoolVar(&tlsInsecure, "tls-no-verify", false, "Disable TLS validation")
	namespacesCmd.Flags().StringVarP(&token, "token", "k", "", "Pass a JWT token to use instead of basic auth")

	// 注册命令
	faasCmd.AddCommand(namespacesCmd)
	namespaceCmd.AddCommand(namespaceListCmd)
}

// namespacesCmd 旧版命名空间列出命令
// 已废弃，仅做兼容，推荐使用 faas-cli namespace list
var namespacesCmd = &cobra.Command{
	Use:   `namespaces [--gateway GATEWAY_URL] [--tls-no-verify] [--token JWT_TOKEN]`,
	Short: "List OpenFaaS namespaces",
	Long:  `Lists OpenFaaS namespaces for the given gateway URL`,
	Example: `  faas-cli namespaces
  faas-cli namespaces --gateway https://127.0.0.1:8080`,
	RunE:       runNamespaces,                                    // 执行逻辑
	Hidden:     true,                                             // 隐藏不显示在帮助中
	Deprecated: "This has moved to \"faas-cli namespace list\".", // 废弃提示
}

// namespaceListCmd 新版「命名空间列表」子命令
// 标准用法：faas-cli namespace list
var namespaceListCmd = &cobra.Command{
	Use:     `list`,
	Aliases: []string{"ls"}, // 支持简写 ls
	Short:   "List OpenFaaS namespaces",
	Long:    `Lists OpenFaaS namespaces for the given gateway URL`,
	Example: `faas-cli namespace list`,
	RunE:    runNamespaces, // 共用同一个执行函数
}

// runNamespaces 列出命名空间的核心逻辑
// 调用 OpenFaaS SDK 获取命名空间列表并打印
func runNamespaces(cmd *cobra.Command, args []string) error {
	// 获取默认 SDK 客户端（已处理网关、认证、TLS）
	client, err := GetDefaultSDKClient()
	if err != nil {
		return err
	}

	// 从服务端查询命名空间列表
	namespaces, err := client.GetNamespaces(context.Background())
	if err != nil {
		return err
	}

	// 格式化打印输出
	printNamespaces(namespaces)
	return nil
}

// printNamespaces 格式化打印命名空间列表
func printNamespaces(namespaces []string) {
	fmt.Print("Namespaces:\n")
	for _, v := range namespaces {
		fmt.Printf(" - %s\n", v)
	}
}
