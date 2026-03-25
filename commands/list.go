// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/openfaas/faas-cli/proxy"
	"github.com/openfaas/faas-provider/types"
	"github.com/openfaas/go-sdk/stack"
	"github.com/spf13/cobra"
)

// 全局命令行标志变量
var (
	verboseList bool   // 详细输出模式
	token       string // JWT 认证令牌
	sortOrder   string // 排序方式：name/invocations/creation
)

func init() {
	// 配置命令行参数
	listCmd.Flags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL starting with http(s)://")
	listCmd.Flags().StringVarP(&functionNamespace, "namespace", "n", "", "Namespace of the function")
	listCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Quiet mode - print out only the function's ID")

	listCmd.Flags().BoolVarP(&verboseList, "verbose", "v", false, "Verbose output for the function list")
	listCmd.Flags().BoolVar(&tlsInsecure, "tls-no-verify", false, "Disable TLS validation")
	listCmd.Flags().BoolVar(&envsubst, "envsubst", true, "Substitute environment variables in stack.yaml file")
	listCmd.Flags().StringVarP(&token, "token", "k", "", "Pass a JWT token to use instead of basic auth")
	listCmd.Flags().StringVar(&sortOrder, "sort", "name", "Sort the functions by \"name\" or \"invocations\"")

	// 将 list 命令添加到根命令
	faasCmd.AddCommand(listCmd)
}

// listCmd 列出已部署的 OpenFaaS 函数
var listCmd = &cobra.Command{
	Use:     `list [--gateway GATEWAY_URL] [--verbose] [--tls-no-verify]`,
	Aliases: []string{"ls"}, // 别名：faas-cli ls
	Short:   "List OpenFaaS functions",
	Long:    `Lists OpenFaaS functions either on a local or remote gateway`,
	Example: `  faas-cli list
  faas-cli list --gateway https://127.0.0.1:8080 --verbose`,
	RunE: runList,
}

// runList 执行函数列表查询逻辑
func runList(cmd *cobra.Command, args []string) error {
	var services stack.Services
	var gatewayAddress string
	var yamlGateway string

	// 如果指定了 yaml 文件，则解析并获取其中的网关地址
	if len(yamlFile) > 0 {
		parsedServices, err := stack.ParseYAMLFile(yamlFile, regex, filter, envsubst)
		if err != nil {
			return err
		}

		if parsedServices != nil {
			services = *parsedServices
			yamlGateway = services.Provider.GatewayURL
		}
	}

	// 获取最终使用的网关地址
	gatewayAddress = getGatewayURL(gateway, defaultGateway, yamlGateway, os.Getenv(openFaaSURLEnvironment))

	// 创建认证客户端
	cliAuth, err := proxy.NewCLIAuth(token, gatewayAddress)
	if err != nil {
		return err
	}
	transport := GetDefaultCLITransport(tlsInsecure, &commandTimeout)
	proxyClient, err := proxy.NewClient(cliAuth, gatewayAddress, transport, &commandTimeout)
	if err != nil {
		return err
	}

	// 请求网关获取函数列表
	functions, err := proxyClient.ListFunctions(context.Background(), functionNamespace)
	if err != nil {
		return err
	}

	// 根据指定方式排序
	switch sortOrder {
	case "name":
		sort.Sort(byName(functions))
	case "invocations":
		sort.Sort(byInvocations(functions))
	case "creation":
		sort.Sort(byCreation(functions))
	}

	// 三种输出格式：安静模式 / 详细模式 / 简洁模式
	if quiet {
		// 安静模式：只打印函数名
		for _, function := range functions {
			fmt.Printf("%s\n", function.Name)
		}
	} else if verboseList {
		// 详细模式：显示函数名、镜像、调用次数、副本数、创建时间

		// 动态计算镜像列的最大宽度
		maxWidth := 40
		for _, function := range functions {
			if len(function.Image) > maxWidth {
				maxWidth = len(function.Image)
			}
		}

		// 打印表头
		fmt.Printf("%-30s\t%-"+fmt.Sprintf("%d", maxWidth)+"s\t%-15s\t%-5s\t%-5s\n",
			"Function", "Image", "Invocations", "Replicas", "CreatedAt")

		// 打印每一行
		for _, function := range functions {
			fmt.Printf("%-30s\t%-"+fmt.Sprintf("%d", maxWidth)+"s\t%-15d\t%-5d\t\t%-5s\n",
				function.Name,
				function.Image,
				int64(function.InvocationCount),
				function.Replicas,
				function.CreatedAt.String())
		}
	} else {
		// 简洁模式（默认）：函数名、调用次数、副本数
		fmt.Printf("%-30s\t%-15s\t%-5s\n", "Function", "Invocations", "Replicas")
		for _, function := range functions {
			fmt.Printf("%-30s\t%-15d\t%-5d\n",
				function.Name,
				int64(function.InvocationCount),
				function.Replicas)
		}
	}

	return nil
}

// byName 按函数名字母序排序
type byName []types.FunctionStatus

func (a byName) Len() int           { return len(a) }
func (a byName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byName) Less(i, j int) bool { return a[i].Name < a[j].Name }

// byInvocations 按调用次数降序排序
type byInvocations []types.FunctionStatus

func (a byInvocations) Len() int           { return len(a) }
func (a byInvocations) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byInvocations) Less(i, j int) bool { return a[i].InvocationCount > a[j].InvocationCount }

// byCreation 按创建时间升序排序（最早在前）
type byCreation []types.FunctionStatus

func (a byCreation) Len() int           { return len(a) }
func (a byCreation) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byCreation) Less(i, j int) bool { return a[i].CreatedAt.Before(a[j].CreatedAt) }
