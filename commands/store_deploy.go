// Copyright (c) OpenFaaS Author(s) 2018. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 store deploy 命令，用于从函数商店一键部署函数
package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/openfaas/faas-cli/util"

	"github.com/openfaas/faas-cli/proxy"
	"github.com/spf13/cobra"
)

// init 初始化 store deploy 命令，注册所有命令行参数并添加到 store 根命令
func init() {
	// Setup flags that are used by multiple commands (variables defined in faas.go)
	storeDeployCmd.Flags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL starting with http(s)://")
	storeDeployCmd.Flags().StringVar(&functionName, "name", "", "Name of the deployed function (overriding name from the store)")
	storeDeployCmd.Flags().StringVarP(&functionNamespace, "namespace", "n", "", "Namespace of the function")
	// Setup flags that are used only by deploy command (variables defined above)
	storeDeployCmd.Flags().StringArrayVarP(&storeDeployFlags.envvarOpts, "env", "e", []string{}, "Adds one or more environment variables to the defined ones by store (ENVVAR=VALUE)")
	storeDeployCmd.Flags().StringArrayVarP(&storeDeployFlags.labelOpts, "label", "l", []string{}, "Set one or more label (LABEL=VALUE)")
	storeDeployCmd.Flags().BoolVar(&storeDeployFlags.replace, "replace", false, "Replace any existing function")
	storeDeployCmd.Flags().BoolVar(&storeDeployFlags.update, "update", true, "Update existing functions")
	storeDeployCmd.Flags().StringArrayVar(&storeDeployFlags.constraints, "constraint", []string{}, "Apply a constraint to the function")
	storeDeployCmd.Flags().StringArrayVar(&storeDeployFlags.secrets, "secret", []string{}, "Give the function access to a secure secret")
	storeDeployCmd.Flags().StringArrayVarP(&storeDeployFlags.annotationOpts, "annotation", "", []string{}, "Set one or more annotation (ANNOTATION=VALUE)")
	storeDeployCmd.Flags().BoolVar(&tlsInsecure, "tls-no-verify", false, "Disable TLS validation")
	storeDeployCmd.Flags().StringVarP(&token, "token", "k", "", "Pass a JWT token to use instead of basic auth")
	storeDeployCmd.Flags().DurationVar(&timeoutOverride, "timeout", commandTimeout, "Timeout for any HTTP calls made to the OpenFaaS API.")

	storeDeployCmd.Flags().StringVar(&cpuRequest, "cpu-request", "", "Supply the CPU request for the function in Mi")
	storeDeployCmd.Flags().StringVar(&cpuLimit, "cpu-limit", "", "Supply the CPU limit for the function in Mi")
	storeDeployCmd.Flags().StringVar(&memoryRequest, "memory-request", "", "Supply the memory request for the function in Mi")
	storeDeployCmd.Flags().StringVar(&memoryLimit, "memory-limit", "", "Supply the memory limit for the function in Mi")

	// Set bash-completion.
	_ = storeDeployCmd.Flags().SetAnnotation("handler", cobra.BashCompSubdirsInDir, []string{})

	storeCmd.AddCommand(storeDeployCmd)
}

// storeDeployCmd 从函数商店部署函数到 OpenFaaS 网关
// 继承商店中定义的环境变量、标签、注解，并支持命令行覆盖
var storeDeployCmd = &cobra.Command{
	Use: `deploy (FUNCTION_NAME|FUNCTION_TITLE)
			[--name FUNCTION_NAME]
			[--gateway GATEWAY_URL]
			[--env ENVVAR=VALUE ...]
			[--label LABEL=VALUE ...]
			[--annotation ANNOTATION=VALUE ...]
			[--replace=false]
			[--update=true]
			[--constraint PLACEMENT_CONSTRAINT ...]
			[--secret "SECRET_NAME"]
			[--url STORE_URL]
			[--tls-no-verify=false]`,

	Short: "Deploy OpenFaaS functions from a store",
	Long:  `Same as faas-cli deploy except that function is pre-loaded with arguments from the store`,
	Example: `  faas-cli store deploy figlet
  faas-cli store deploy figlet \
    --gateway=http://127.0.0.1:8080 \
    --env=MYVAR=myval`,
	RunE:    runStoreDeploy,
	PreRunE: preRunEStoreDeploy,
}

// preRunEStoreDeploy 部署前校验：必须传入函数名称
func preRunEStoreDeploy(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("please provide the function name")
	}

	return nil
}

// runStoreDeploy 执行从商店部署函数的核心逻辑
// 1. 获取目标平台与商店地址
// 2. 获取并过滤函数列表
// 3. 合并商店与命令行的环境变量、标签、注解
// 4. 调用底层 deployImage 完成部署
func runStoreDeploy(cmd *cobra.Command, args []string) error {
	targetPlatform := getTargetPlatform(platformValue)

	// 环境变量优先级：未指定 --url 则使用 OPENFAAS_STORE
	if v, ok := os.LookupEnv("OPENFAAS_STORE"); ok && len(v) > 0 {
		if !storeCmd.Flags().Changed("url") {
			storeAddress = v
		}
	}

	storeItems, err := storeList(storeAddress)
	if err != nil {
		return err
	}

	platformFunctions := filterStoreList(storeItems, targetPlatform)

	requestedStoreFn := args[0]
	item := storeFindFunction(requestedStoreFn, platformFunctions)
	if item == nil {
		return fmt.Errorf("function '%s' not found for platform '%s'", requestedStoreFn, targetPlatform)
	}

	// 解析命令行传入的环境变量
	flagEnvs, err := util.ParseMap(storeDeployFlags.envvarOpts, "env")
	if err != nil {
		return err
	}

	// 合并商店环境变量 + 命令行变量（命令行优先）
	mergedEnvs := util.MergeMap(item.Environment, flagEnvs)

	envs := []string{}
	for k, v := range mergedEnvs {
		env := fmt.Sprintf("%s=%s", k, v)
		envs = append(envs, env)
	}
	storeDeployFlags.envvarOpts = envs

	// 合并商店标签
	if item.Labels != nil {
		for k, v := range item.Labels {
			label := fmt.Sprintf("%s=%s", k, v)
			storeDeployFlags.labelOpts = append(storeDeployFlags.labelOpts, label)
		}
	}

	// 合并商店注解
	if item.Annotations != nil {
		for k, v := range item.Annotations {
			annotation := fmt.Sprintf("%s=%s", k, v)
			storeDeployFlags.annotationOpts = append(storeDeployFlags.annotationOpts, annotation)
		}
	}

	// 最终函数名：命令行指定 > 商店名称
	itemName := item.Name
	if functionName != "" {
		itemName = functionName
	}

	// 获取对应平台的函数镜像
	imageName := item.GetImageName(targetPlatform)

	// 创建网关客户端
	gateway = getGatewayURL(gateway, defaultGateway, "", os.Getenv(openFaaSURLEnvironment))
	cliAuth, err := proxy.NewCLIAuth(token, gateway)
	if err != nil {
		return err
	}
	transport := GetDefaultCLITransport(tlsInsecure, &timeoutOverride)
	proxyClient, err := proxy.NewClient(cliAuth, gateway, transport, &timeoutOverride)
	if err != nil {
		return err
	}

	// 执行部署
	statusCode, err := deployImage(context.Background(),
		proxyClient,
		imageName,
		item.Fprocess,
		itemName,
		"",
		storeDeployFlags,
		tlsInsecure,
		item.ReadOnlyRootFilesystem,
		token,
		functionNamespace,
		cpuRequest,
		cpuLimit,
		memoryRequest,
		memoryLimit)

	// 检查部署状态码
	if badStatusCode(statusCode) {
		failedStatusCode := map[string]int{itemName: statusCode}
		err := deployFailed(failedStatusCode)
		return err
	}

	return err
}
