// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 remove 命令，用于删除已部署的 OpenFaaS 函数
package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/openfaas/faas-cli/proxy"
	"github.com/openfaas/go-sdk/stack"
	"github.com/spf13/cobra"
)

// init 初始化 remove 命令，注册命令行参数并添加到主命令 faasCmd
func init() {
	// Setup flags that are used by multiple commands (variables defined in faas.go)
	removeCmd.Flags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL starting with http(s)://")
	removeCmd.Flags().BoolVar(&tlsInsecure, "tls-no-verify", false, "Disable TLS validation")
	removeCmd.Flags().BoolVar(&envsubst, "envsubst", true, "Substitute environment variables in stack.yaml file")
	removeCmd.Flags().StringVarP(&token, "token", "k", "", "Pass a JWT token to use instead of basic auth")
	removeCmd.Flags().StringVarP(&functionNamespace, "namespace", "n", "", "Namespace of the function")

	faasCmd.AddCommand(removeCmd)
}

// removeCmd 删除/移除已部署的 OpenFaaS 函数
// 支持别名 rm / delete，可通过函数名或 stack.yaml 文件批量删除
var removeCmd = &cobra.Command{
	Use: `remove FUNCTION_NAME [--gateway GATEWAY_URL]
  faas-cli remove -f YAML_FILE [--regex "REGEX"] [--filter "WILDCARD"]`,
	Aliases: []string{"rm", "delete"},
	Short:   "Remove deployed OpenFaaS functions",
	Long: `Removes/deletes deployed OpenFaaS functions either via the supplied YAML config
using the "--yaml" flag (which may contain multiple function definitions), or by
explicitly specifying a function name.`,
	Example: `  faas-cli remove -f https://domain/path/myfunctions.yml
  faas-cli remove -f stack.yaml
  faas-cli remove -f stack.yaml --filter "*gif*"
  faas-cli remove -f stack.yaml --regex "fn[0-9]_.*"
  faas-cli remove url-ping
  faas-cli remove img2ansi --gateway==http://remote-site.com:8080`,
	RunE: runDelete,
}

// runDelete 执行删除函数的核心逻辑
// 1. 解析 stack.yaml 文件（如果指定）
// 2. 确定目标网关地址
// 3. 创建 API 客户端
// 4. 批量删除 YAML 中的函数 或 删除单个指定函数
func runDelete(cmd *cobra.Command, args []string) error {
	var services stack.Services
	var gatewayAddress string
	var yamlGateway string

	// 1. 解析 stack.yaml 配置文件
	if len(yamlFile) > 0 && len(args) == 0 {
		parsedServices, err := stack.ParseYAMLFile(yamlFile, regex, filter, envsubst)
		if err != nil {
			return err
		}

		if parsedServices != nil {
			services = *parsedServices
			yamlGateway = services.Provider.GatewayURL
		}
	}

	// 2. 获取最终使用的网关地址
	gatewayAddress = getGatewayURL(gateway, defaultGateway, yamlGateway, os.Getenv(openFaaSURLEnvironment))

	// 3. 创建 OpenFaaS 客户端
	cliAuth, err := proxy.NewCLIAuth(token, gatewayAddress)
	if err != nil {
		return err
	}
	transport := GetDefaultCLITransport(tlsInsecure, &commandTimeout)
	proxyclient, err := proxy.NewClient(cliAuth, gatewayAddress, transport, &commandTimeout)
	if err != nil {
		return err
	}
	ctx := context.Background()

	// 4. 批量删除 stack.yaml 中的函数
	if len(services.Functions) > 0 {
		for k, function := range services.Functions {
			// 确定函数命名空间
			function.Namespace = getNamespace(functionNamespace, function.Namespace)
			function.Name = k
			fmt.Printf("Deleting: %s.%s\n", function.Name, function.Namespace)

			// 调用 API 删除函数
			proxyclient.DeleteFunction(ctx, function.Name, function.Namespace)
		}
	} else {
		// 5. 删除单个指定名称的函数
		if len(args) < 1 {
			return fmt.Errorf("please provide the name of a function to delete")
		}

		functionName = args[0]
		fmt.Printf("Deleting: %s.%s\n", functionName, functionNamespace)
		err := proxyclient.DeleteFunction(ctx, functionName, functionNamespace)
		if err != nil {
			return err
		}
	}

	return nil
}
