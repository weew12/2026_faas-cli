// Copyright (c) OpenFaaS Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 secret list 命令，用于列出当前命名空间下的所有密钥
package commands

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/openfaas/faas-cli/proxy"
	types "github.com/openfaas/faas-provider/types"
	"github.com/spf13/cobra"
)

// secretListCmd 列出所有可用的密钥
// 支持别名 ls，可指定网关、命名空间、认证令牌
var secretListCmd = &cobra.Command{
	Use:     `list [--tls-no-verify]`,
	Aliases: []string{"ls"},
	Short:   "List all secrets",
	Long:    `List all secrets in the specified namespace`,
	Example: `faas-cli secret list
faas-cli secret list --gateway=http://127.0.0.1:8080`,
	RunE:    runSecretList,
	PreRunE: preRunSecretListCmd,
}

// init 初始化 secret list 命令，注册命令行参数并添加到 secret 根命令
func init() {
	secretListCmd.Flags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL starting with http(s)://")
	secretListCmd.Flags().BoolVar(&tlsInsecure, "tls-no-verify", false, "Disable TLS validation")
	secretListCmd.Flags().StringVarP(&token, "token", "k", "", "Pass a JWT token to use instead of basic auth")
	secretListCmd.Flags().StringVarP(&functionNamespace, "namespace", "n", "", "Namespace of the function")

	secretCmd.AddCommand(secretListCmd)
}

// preRunSecretListCmd 前置检查，当前无需校验
func preRunSecretListCmd(cmd *cobra.Command, args []string) error {
	return nil
}

// runSecretList 执行密钥列表查询的核心逻辑
// 1. 获取网关地址
// 2. 创建 OpenFaaS 客户端
// 3. 查询密钥列表
// 4. 格式化并输出结果
func runSecretList(cmd *cobra.Command, args []string) error {
	gatewayAddress := getGatewayURL(gateway, defaultGateway, "", os.Getenv(openFaaSURLEnvironment))

	if msg := checkTLSInsecure(gatewayAddress, tlsInsecure); len(msg) > 0 {
		fmt.Println(msg)
	}

	// 创建 API 客户端
	cliAuth, err := proxy.NewCLIAuth(token, gatewayAddress)
	if err != nil {
		return err
	}
	transport := GetDefaultCLITransport(tlsInsecure, &commandTimeout)
	client, err := proxy.NewClient(cliAuth, gatewayAddress, transport, &commandTimeout)
	if err != nil {
		return err
	}

	// 获取密钥列表
	secrets, err := client.GetSecretList(context.Background(), functionNamespace)
	if err != nil {
		return err
	}

	// 无密钥时提示
	if len(secrets) == 0 {
		fmt.Printf("No secrets found.\n")
		return nil
	}

	// 渲染并输出
	fmt.Printf("%s", renderSecretList(secrets))

	return nil
}

// renderSecretList 将密钥列表格式化为表格形式输出
func renderSecretList(secrets []types.Secret) string {
	var b bytes.Buffer
	w := tabwriter.NewWriter(&b, 0, 0, 1, ' ', 0)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "NAME")

	// 遍历输出所有密钥名称
	for _, secret := range secrets {
		fmt.Fprintf(w, "%s\n", secret.Name)
	}

	fmt.Fprintln(w)
	w.Flush()
	return b.String()
}
