// Copyright (c) OpenFaaS Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 secret update 命令，用于更新函数密钥
package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/openfaas/faas-cli/proxy"
	types "github.com/openfaas/faas-provider/types"
	"github.com/spf13/cobra"
)

// secretUpdateCmd 更新已存在的密钥
// 支持从字面量、文件、标准输入读取密钥值
var secretUpdateCmd = &cobra.Command{
	Use:     "update [--tls-no-verify]",
	Aliases: []string{"u"},
	Short:   "Update a secret",
	Long:    `Update a secret by name`,
	Example: `faas-cli secret update NAME
faas-cli secret update NAME --from-literal=secret-value
faas-cli secret update NAME --from-file=/path/to/secret/file
faas-cli secret update NAME --from-file=/path/to/secret/file --trim=false
faas-cli secret update NAME --from-literal=secret-value --gateway=http://127.0.0.1:8080
cat /path/to/secret/file | faas-cli secret update NAME`,
	RunE:    runSecretUpdate,
	PreRunE: preRunSecretUpdate,
}

// init 初始化 secret update 命令，注册命令行参数并添加到 secret 根命令
func init() {
	secretUpdateCmd.Flags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL starting with http(s)://")
	secretUpdateCmd.Flags().BoolVar(&tlsInsecure, "tls-no-verify", false, "Disable TLS validation")
	secretUpdateCmd.Flags().StringVar(&literalSecret, "from-literal", "", "Value of the secret")
	secretUpdateCmd.Flags().StringVar(&secretFile, "from-file", "", "Path to the secret file")
	secretUpdateCmd.Flags().BoolVar(&trimSecret, "trim", true, "trim whitespace from the start and end of the secret value")
	secretUpdateCmd.Flags().StringVarP(&token, "token", "k", "", "Pass a JWT token to use instead of basic auth")
	secretUpdateCmd.Flags().StringVarP(&functionNamespace, "namespace", "n", "", "Namespace of the function")
	secretCmd.AddCommand(secretUpdateCmd)
}

// preRunSecretUpdate 命令执行前的参数校验
// 1. 必须传入密钥名称
// 2. 名称只能有一个
// 3. 不能同时使用多个密钥输入方式
func preRunSecretUpdate(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("secret name required")
	}

	if len(args) > 1 {
		return fmt.Errorf("too many values for secret name")
	}

	if len(secretFile) > 0 && len(literalSecret) > 0 {
		return fmt.Errorf("please provide secret using only one option from --from-literal, --from-file and STDIN")
	}

	return nil
}

// runSecretUpdate 执行密钥更新逻辑
// 1. 获取网关地址
// 2. 读取密钥值（来自 literal/file/stdin）
// 3. 去除首尾空白（trim 开启时）
// 4. 调用 OpenFaaS 客户端更新密钥
func runSecretUpdate(cmd *cobra.Command, args []string) error {
	gatewayAddress := getGatewayURL(gateway, defaultGateway, "", os.Getenv(openFaaSURLEnvironment))

	if msg := checkTLSInsecure(gatewayAddress, tlsInsecure); len(msg) > 0 {
		fmt.Println(msg)
	}

	secret := types.Secret{
		Name:      args[0],
		Namespace: functionNamespace,
	}

	// 从三种方式读取密钥值
	switch {
	case len(literalSecret) > 0:
		secret.Value = literalSecret

	case len(secretFile) > 0:
		content, err := os.ReadFile(secretFile)
		if err != nil {
			return fmt.Errorf("unable to read secret file: %s", err.Error())
		}
		secret.Value = string(content)

	default:
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			fmt.Fprintf(os.Stderr, "Reading from STDIN - hit (Control + D) to stop.\n")
		}

		secretStdin, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("unable to read standard input: %s", err.Error())
		}
		secret.Value = string(secretStdin)
	}

	// 去除首尾空白字符
	if trimSecret {
		secret.Value = strings.TrimSpace(secret.Value)
	}

	// 密钥值不能为空
	if len(secret.Value) == 0 {
		return fmt.Errorf("must provide a non empty secret via --from-literal, --from-file or STDIN")
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

	fmt.Println("Updating secret: " + secret.Name)
	// 调用 API 更新密钥
	_, output := client.UpdateSecret(context.Background(), secret)
	fmt.Printf("%s", output)

	return nil
}
