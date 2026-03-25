// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 ready 命令，用于阻塞等待网关或函数变为就绪状态
package commands

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/openfaas/faas-cli/proxy"
	"github.com/openfaas/go-sdk/stack"
	"github.com/spf13/cobra"
)

// init 初始化 ready 命令，注册命令行参数并添加到主命令 faasCmd
func init() {
	// Setup flags that are used by multiple commands (variables defined in faas.go)
	readyCmd.Flags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL starting with http(s)://")
	readyCmd.Flags().StringVarP(&functionNamespace, "namespace", "n", "", "Namespace of the function")
	readyCmd.Flags().BoolVar(&tlsInsecure, "tls-no-verify", false, "Disable TLS validation")

	readyCmd.Flags().Int("attempts", 60, "Number of attempts to check the gateway")
	readyCmd.Flags().Duration("interval", time.Second*1, "Interval between attempts in seconds")

	faasCmd.AddCommand(readyCmd)
}

// readyCmd 阻塞程序执行，直到网关健康 或 函数可用副本数 > 0
// 可用于脚本中等待部署完成后再执行后续操作
var readyCmd = &cobra.Command{
	Use:   `ready [--gateway GATEWAY_URL] [--tls-no-verify] [FUNCTION_NAME]`,
	Short: "Block until the gateway or a function is ready for use",
	Example: `  # Block until the gateway is ready
  faas-cli ready --gateway https://127.0.0.1:8080

  # Block until the env function is ready
  faas-cli store deploy env && \
    faas-cli ready env

  # Block until the env function is ready in staging-fn namespace
  faas-cli store deploy env --namespace staging-fn && \
    faas-cli ready env --namespace staging-fn
`,
	RunE: runReadyCmd,
}

// runReadyCmd 执行等待就绪的核心逻辑
// 分支1：无函数名 → 等待网关 /healthz 返回 200
// 分支2：有函数名 → 等待函数 AvailableReplicas > 0
func runReadyCmd(cmd *cobra.Command, args []string) error {
	interval, err := cmd.Flags().GetDuration("interval")
	if err != nil {
		return err
	}

	attempts, err := cmd.Flags().GetInt("attempts")
	if err != nil {
		return err
	}

	if attempts < 1 {
		return fmt.Errorf("attempts must be greater than 0")
	}

	var services stack.Services
	var gatewayAddress string
	var yamlGateway string
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
	gatewayAddress = getGatewayURL(gateway, defaultGateway, yamlGateway, os.Getenv(openFaaSURLEnvironment))
	transport := GetDefaultCLITransport(tlsInsecure, &commandTimeout)

	// 分支1：不指定函数名 → 等待网关健康检查就绪
	if len(args) == 0 {
		ready := false

		c := &http.Client{
			Transport: transport,
		}

		u, err := url.Parse(gatewayAddress)
		if err != nil {
			return err
		}

		u.Path = "/healthz"

		for i := 0; i < attempts; i++ {
			fmt.Printf("[%d/%d] Waiting for gateway\n", i+1, attempts)
			req, err := http.NewRequest(http.MethodGet, u.String(), nil)
			if err != nil {
				return err
			}

			res, err := c.Do(req)
			if err != nil {
				fmt.Printf("[%d/%d] Error reaching OpenFaaS gateway: %s\n", i+1, attempts, err.Error())
			} else if res.StatusCode == http.StatusOK {
				fmt.Printf("OpenFaaS gateway is ready\n")
				ready = true
				break
			}

			time.Sleep(interval)
		}

		if !ready {
			return fmt.Errorf("gateway: %s not ready after: %s", gatewayAddress, interval*time.Duration(attempts).Round(time.Second))
		}

	} else {
		// 分支2：指定函数名 → 等待函数可用副本数 > 0
		functionName := args[0]
		ready := false
		cliAuth, err := proxy.NewCLIAuth(token, gatewayAddress)
		if err != nil {
			return err
		}

		cliClient, err := proxy.NewClient(cliAuth, gatewayAddress, transport, &commandTimeout)
		if err != nil {
			return err
		}

		ctx := context.Background()

		for i := 0; i < attempts; i++ {
			suffix := ""
			if len(functionNamespace) > 0 {
				suffix = "." + functionNamespace
			}

			fmt.Printf("[%d/%d] Waiting for function %s%s\n", i+1, attempts, functionName, suffix)

			function, err := cliClient.GetFunctionInfo(ctx, functionName, functionNamespace)
			if err != nil {
				fmt.Printf("[%d/%d] Error getting function info: %s\n", i+1, attempts, err.Error())
			}

			if function.AvailableReplicas > 0 {
				fmt.Printf("Function %s is ready\n", functionName)
				ready = true
				break
			}
			time.Sleep(interval)
		}

		if !ready {
			return fmt.Errorf("function %s not ready after: %s", functionName, interval*time.Duration(attempts).Round(time.Second))
		}

	}

	return nil
}
