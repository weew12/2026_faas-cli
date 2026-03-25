// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 所有命令行功能
// 本文件实现 version 命令，用于打印 CLI 与服务端版本信息、检查更新
package commands

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"os"

	"github.com/alexellis/arkade/pkg/get"
	"github.com/morikuni/aec"
	"github.com/openfaas/faas-cli/proxy"
	"github.com/openfaas/faas-cli/version"
	"github.com/openfaas/go-sdk/stack"
	"github.com/spf13/cobra"
)

// GitCommit 编译时注入的 Git 提交哈希，用于标识构建版本
var (
	shortVersion bool // 仅打印简短版本号
	warnUpdate   bool // 是否检查并提示版本更新
)

// init 初始化 version 命令，注册命令行标志并添加到根命令
func init() {
	versionCmd.Flags().BoolVar(&shortVersion, "short-version", false, "Just print Git SHA")
	versionCmd.Flags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL starting with http(s)://")
	versionCmd.Flags().BoolVar(&tlsInsecure, "tls-no-verify", false, "Disable TLS validation")
	versionCmd.Flags().BoolVar(&envsubst, "envsubst", true, "Substitute environment variables in stack.yaml file")

	versionCmd.Flags().BoolVar(&warnUpdate, "warn-update", true, "Check for new version and warn about updating")

	versionCmd.Flags().StringVarP(&token, "token", "k", "", "Pass a JWT token to use instead of basic auth")
	faasCmd.AddCommand(versionCmd)
}

// versionCmd 定义 version 子命令，用于展示客户端与服务端版本信息
var versionCmd = &cobra.Command{
	Use:   "version [--short-version] [--gateway GATEWAY_URL]",
	Short: "Display the clients version information",
	Long: fmt.Sprintf(`The version command returns the current clients version information.

This currently consists of the GitSHA from which the client was built.
- https://github.com/openfaas/faas-cli/tree/%s`, version.GitCommit),
	Example: `  faas-cli version
  faas-cli version --short-version`,
	RunE: runVersionE,
}

// runVersionE 执行 version 命令的核心逻辑
// 打印 CLI 版本、Logo、服务端版本，并检查最新版本提示更新
func runVersionE(cmd *cobra.Command, args []string) error {
	// 仅输出简短版本
	if shortVersion {
		fmt.Println(version.BuildVersion())
		return nil
	}

	// 打印彩色 Logo
	printLogo()
	// 打印 CLI 构建信息
	fmt.Printf(`CLI:
 commit:  %s
 version: %s
`, version.GitCommit, version.BuildVersion())
	// 打印网关与服务端版本信息
	printServerVersions()

	// 检查并提示版本更新
	if warnUpdate {
		currentVersion := version.Version
		latestVersion, err := get.FindGitHubRelease("openfaas", "faas-cli")
		if err != nil {
			return fmt.Errorf("unable to find latest version online error: %s", err.Error())
		}

		if currentVersion != "" && currentVersion != latestVersion {
			fmt.Printf("Your faas-cli version (%s) may be out of date. Version: %s is now available on GitHub.\n", currentVersion, latestVersion)
		}
	}

	return nil
}

// printServerVersions 获取并打印 OpenFaaS 网关与服务端版本信息
// 从 stack.yaml 或命令行参数获取网关地址，调用 API 获取服务端信息
func printServerVersions() error {
	var services stack.Services
	var gatewayAddress string
	var yamlGateway string

	// 解析 stack.yaml 获取配置的网关地址
	if len(yamlFile) > 0 {
		parsedServices, err := stack.ParseYAMLFile(yamlFile, regex, filter, envsubst)
		if err == nil && parsedServices != nil {
			services = *parsedServices
			yamlGateway = services.Provider.GatewayURL
		}
	}

	// 确定最终使用的网关地址
	gatewayAddress = getGatewayURL(gateway, defaultGateway, yamlGateway, os.Getenv(openFaaSURLEnvironment))

	// 创建 API 客户端请求服务端信息
	versionTimeout := 5 * time.Second
	cliAuth, err := proxy.NewCLIAuth(token, gatewayAddress)
	if err != nil {
		return err
	}
	transport := GetDefaultCLITransport(tlsInsecure, &versionTimeout)
	cliClient, err := proxy.NewClient(cliAuth, gatewayAddress, transport, &versionTimeout)
	if err != nil {
		return err
	}
	gatewayInfo, err := cliClient.GetSystemInfo(context.Background())
	if err != nil {
		return err
	}

	// 打印网关详情
	printGatewayDetails(gatewayAddress, gatewayInfo.Version.Release, gatewayInfo.Version.SHA)

	// 打印服务提供方信息
	fmt.Printf(`
Provider
 name:          %s
 orchestration: %s
 version:       %s 
 sha:           %s
`, gatewayInfo.Provider.Name, gatewayInfo.Provider.Orchestration, gatewayInfo.Provider.Version.Release, gatewayInfo.Provider.Version.SHA)
	return nil
}

// printGatewayDetails 格式化打印网关地址、版本、提交哈希
func printGatewayDetails(gatewayAddress, version, sha string) {
	fmt.Printf(`
Gateway
 uri:     %s`, gatewayAddress)

	if version != "" {
		fmt.Printf(`
 version: %s
 sha:     %s
`, version, sha)
	}

	fmt.Println()
}

// printLogo 打印 OpenFaaS 彩色 ASCII 标志
// Windows 系统使用绿色，其他系统使用蓝色
func printLogo() {
	figletColoured := aec.BlueF.Apply(figletStr)
	if runtime.GOOS == "windows" {
		figletColoured = aec.GreenF.Apply(figletStr)
	}
	fmt.Printf("%s", figletColoured)
}

// figletStr ASCII 艺术字 Logo，使用 figlet 生成
const figletStr = `  ___                   _____           ____
 / _ \ _ __   ___ _ __ |  ___|_ _  __ _/ ___|
| | | | '_ \ / _ \ '_ \| |_ / _` + "`" + ` |/ _` + "`" + ` \___ \
| |_| | |_) |  __/ | | |  _| (_| | (_| |___) |
 \___/| .__/ \___|_| |_|_|  \__,_|\__,_|____/
      |_|

`
