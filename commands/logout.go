// Copyright (c) OpenFaaS Author(s) 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/openfaas/faas-cli/config"
	"github.com/spf13/cobra"
)

// init 初始化 logout 命令，注册网关参数并添加到主命令
func init() {
	logoutCmd.Flags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL starting with http(s)://")

	faasCmd.AddCommand(logoutCmd)
}

// logoutCmd 登出命令：清除本地保存的网关认证信息
var logoutCmd = &cobra.Command{
	Use:     `logout [--gateway GATEWAY_URL]`,
	Short:   "Log out from OpenFaaS gateway",
	Long:    "Log out from OpenFaaS gateway.\nIf no gateway is specified, the default local one will be used.",
	Example: `  faas-cli logout --gateway https://openfaas.mydomain.com`,
	RunE:    runLogout,
}

// runLogout 执行登出逻辑：删除指定网关的认证配置
func runLogout(cmd *cobra.Command, args []string) error {
	// 校验网关地址不能为空
	if len(gateway) == 0 {
		return fmt.Errorf("gateway cannot be an empty string")
	}

	// 格式化并标准化网关 URL
	gateway = strings.TrimSpace(gateway)
	gateway = getGatewayURL(gateway, defaultGateway, "", os.Getenv(openFaaSURLEnvironment))

	// 调用配置包删除认证信息
	err := config.RemoveAuthConfig(gateway)
	if err != nil {
		return err
	}

	fmt.Println("credentials removed for", gateway)

	return nil
}
