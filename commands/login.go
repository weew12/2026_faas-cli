// Copyright (c) OpenFaaS Author(s) 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the root for full license information.

package commands

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/openfaas/faas-cli/proxy"

	"github.com/openfaas/faas-cli/config"
	"github.com/spf13/cobra"
)

// 全局命令行参数变量
var (
	username      string // 用户名
	password      string // 密码
	passwordStdin bool   // 是否从标准输入读取密码
)

// init 初始化 login 命令，注册所有标志并添加到主命令
func init() {
	loginCmd.Flags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL starting with http(s)://")
	loginCmd.Flags().StringVarP(&username, "username", "u", "admin", "Gateway username")
	loginCmd.Flags().StringVarP(&password, "password", "p", "", "Gateway password")
	loginCmd.Flags().BoolVarP(&passwordStdin, "password-stdin", "s", false, "Reads the gateway password from stdin")
	loginCmd.Flags().BoolVar(&tlsInsecure, "tls-no-verify", false, "Disable TLS validation")
	loginCmd.Flags().Duration("timeout", time.Second*5, "Override the timeout for this API call")

	faasCmd.AddCommand(loginCmd)
}

// loginCmd 登录 OpenFaaS 网关，保存认证信息到本地配置
var loginCmd = &cobra.Command{
	Use:   `login [--username admin|USERNAME] [--password PASSWORD] [--gateway GATEWAY_URL] [--tls-no-verify]`,
	Short: "Log in to OpenFaaS gateway",
	Long:  "Log in to OpenFaaS gateway.\nIf no gateway is specified, the default value will be used.",
	Example: `  cat ~/faas_pass.txt | faas-cli login -u user --password-stdin
  echo $PASSWORD | faas-cli login -s  --gateway https://openfaas.mydomain.com
  faas-cli login -u user -p password`,
	RunE: runLogin,
}

// runLogin 执行登录逻辑
func runLogin(cmd *cobra.Command, args []string) error {

	// 获取请求超时时间
	timeout, err := cmd.Flags().GetDuration("timeout")
	if err != nil {
		return err
	}

	// 校验用户名不能为空
	if len(username) == 0 {
		return fmt.Errorf("must provide --username or -u")
	}

	// 处理 --password 参数
	if len(password) > 0 {
		// 警告：直接使用 --password 不安全
		fmt.Println("WARNING! Using --password is insecure, consider using: cat ~/faas_pass.txt | faas-cli login -u user --password-stdin")
		// --password 和 --password-stdin 不能同时使用
		if passwordStdin {
			return fmt.Errorf("--password and --password-stdin are mutually exclusive")
		}
	}

	// 处理 --password-stdin 从标准输入读取密码
	if passwordStdin {
		passwordStdinBytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		// 去除空格和换行符
		password = strings.TrimSpace(string(passwordStdinBytes))
	}

	// 密码去空格并校验非空
	password = strings.TrimSpace(password)
	if len(password) == 0 {
		return fmt.Errorf("must provide a non-empty password via --password or --password-stdin")
	}

	fmt.Println("Calling the OpenFaaS server to validate the credentials...")

	// 标准化网关地址
	gateway = getGatewayURL(gateway, defaultGateway, "", os.Getenv(openFaaSURLEnvironment))

	// 验证用户名密码是否正确
	if err := validateLogin(gateway, username, password, timeout, tlsInsecure); err != nil {
		return err
	}

	// 对用户名密码进行 base64 编码
	token := config.EncodeAuth(username, password)
	// 构造认证配置
	authConfig := config.AuthConfig{
		Gateway: gateway,
		Token:   token,
		Auth:    config.BasicAuthType,
	}
	// 保存认证信息到本地配置文件
	if err := config.UpdateAuthConfig(authConfig); err != nil {
		return err
	}

	// 读取并验证保存的配置
	authConfig, err = config.LookupAuthConfig(gateway)
	if err != nil {
		return err
	}

	// 解码并打印成功信息
	user, _, err := config.DecodeAuth(authConfig.Token)
	if err != nil {
		return err
	}
	fmt.Println("credentials saved for", user, gateway)

	return nil
}

// validateLogin 向网关发送请求验证用户名密码是否有效
func validateLogin(gatewayURL string, user string, pass string, timeout time.Duration, insecureTLS bool) error {

	// 输出 TLS 不安全警告
	if len(checkTLSInsecure(gatewayURL, insecureTLS)) > 0 {
		fmt.Println(NoTLSWarn)
	}

	// 创建 HTTP 客户端
	client := proxy.MakeHTTPClient(&timeout, insecureTLS)
	// 构造请求：访问 /system/functions 测试认证
	req, err := http.NewRequest("GET", gatewayURL+"/system/functions", nil)
	if err != nil {
		return fmt.Errorf("invalid URL: %s", gatewayURL)
	}

	// 设置 Basic Auth
	req.SetBasicAuth(user, pass)
	// 发送请求
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot connect to OpenFaaS on URL: %s. %v", gatewayURL, err)
	}

	// 确保 body 被读取并关闭
	if res.Body != nil {
		defer func() {
			_, _ = io.Copy(io.Discard, res.Body)
			_ = res.Body.Close()
		}()
	}

	// 根据状态码判断结果
	switch res.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return fmt.Errorf("unable to login, either username or password is incorrect")
	default:
		bytesOut, err := io.ReadAll(res.Body)
		if err == nil {
			return fmt.Errorf("server returned unexpected status code: %d - %s", res.StatusCode, string(bytesOut))
		}
	}

	return nil
}
