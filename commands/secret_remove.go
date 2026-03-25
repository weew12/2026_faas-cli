// Copyright (c) OpenFaaS Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 secret remove 命令，用于删除指定名称的密钥
package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/openfaas/faas-cli/proxy"
	types "github.com/openfaas/faas-provider/types"
	"github.com/spf13/cobra"
)

// secretRemoveCmd 删除指定名称的密钥
// 支持别名 rm / delete，可指定网关、命名空间、认证令牌
var secretRemoveCmd = &cobra.Command{
	Use:     "remove [--tls-no-verify]",
	Aliases: []string{"rm", "delete"},
	Short:   "remove a secret",
	Long:    `Remove a secret by name`,
	Example: `faas-cli secret remove NAME
faas-cli secret remove NAME --gateway=http://127.0.0.1:8080`,
	RunE:    runSecretRemove,
	PreRunE: preRunSecretRemoveCmd,
}

// init 初始化命令参数，注册到 secret 根命令
func init() {
	secretRemoveCmd.Flags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL starting with http(s)://")
	secretRemoveCmd.Flags().BoolVar(&tlsInsecure, "tls-no-verify", false, "Disable TLS validation")
	secretRemoveCmd.Flags().StringVarP(&token, "token", "k", "", "Pass a JWT token to use instead of basic auth")
	secretRemoveCmd.Flags().StringVarP(&functionNamespace, "namespace", "n", "", "Namespace of the function")
	secretCmd.AddCommand(secretRemoveCmd)
}

// preRunSecretRemoveCmd 执行前参数校验
// 必须传入且只能传入一个密钥名称
func preRunSecretRemoveCmd(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("secret name required")
	}

	if len(args) > 1 {
		return fmt.Errorf("too many values for secret name")
	}
	return nil
}

// runSecretRemove 执行删除密钥的核心逻辑
// 1. 获取网关地址
// 2. 构造密钥对象
// 3. 创建客户端并调用删除接口
// 4. 输出删除结果
func runSecretRemove(cmd *cobra.Command, args []string) error {
	gatewayAddress := getGatewayURL(gateway, defaultGateway, "", os.Getenv(openFaaSURLEnvironment))

	if msg := checkTLSInsecure(gatewayAddress, tlsInsecure); len(msg) > 0 {
		fmt.Println(msg)
	}

	// 构造待删除的密钥结构
	secret := types.Secret{
		Name:      args[0],
		Namespace: functionNamespace,
	}

	// 创建 OpenFaaS 客户端
	cliAuth, err := proxy.NewCLIAuth(token, gatewayAddress)
	if err != nil {
		return err
	}
	transport := GetDefaultCLITransport(tlsInsecure, &commandTimeout)
	client, err := proxy.NewClient(cliAuth, gatewayAddress, transport, &commandTimeout)
	if err != nil {
		return err
	}

	// 调用 API 删除密钥
	err = client.RemoveSecret(context.Background(), secret)
	if err != nil {
		return err
	}

	fmt.Print("Removed.. OK.\n")

	return nil
}
