// Copyright (c) OpenFaaS Author(s) 2025. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/openfaas/faas-cli/config"
	"github.com/openfaas/go-sdk"
)

// commandTimeout CLI 默认请求超时时间
var (
	commandTimeout = 60 * time.Second
)

// GetDefaultCLITransport 创建自定义 HTTP 传输配置
// 支持：超时设置、禁用 TLS 验证（不安全模式）、代理环境变量
func GetDefaultCLITransport(tlsInsecure bool, timeout *time.Duration) *http.Transport {
	if timeout != nil || tlsInsecure {
		tr := &http.Transport{
			// 使用系统代理（HTTP_PROXY / HTTPS_PROXY）
			Proxy:             http.ProxyFromEnvironment,
			DisableKeepAlives: false, // 启用长连接
		}

		// 如果设置了超时，配置 Dial 超时
		if timeout != nil {
			tr.DialContext = (&net.Dialer{
				Timeout: *timeout,
			}).DialContext

			tr.IdleConnTimeout = 5 * time.Second               // 空闲连接超时
			tr.ExpectContinueTimeout = 1500 * time.Millisecond // 100-continue 超时
		}

		// 允许不安全的 TLS（跳过证书验证）
		if tlsInsecure {
			tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: tlsInsecure}
		}
		tr.DisableKeepAlives = false

		return tr
	}
	// 使用默认传输
	return nil
}

// GetDefaultSDKClient 创建并配置 OpenFaaS Go SDK 客户端
// 自动处理：网关地址、Basic 认证、OAuth2 令牌、命令行令牌、TLS 配置
func GetDefaultSDKClient() (*sdk.Client, error) {
	// 从 stack.yaml 获取网关地址（如果已解析）
	var yamlUrl string
	if services != nil {
		yamlUrl = services.Provider.GatewayURL
	}

	// 获取最终网关地址（优先级：命令行 > yaml > 环境变量 > 默认值）
	gatewayAddress := getGatewayURL(gateway, defaultGateway, yamlUrl, os.Getenv(openFaaSURLEnvironment))
	gatewayURL, err := url.Parse(gatewayAddress)
	if err != nil {
		return nil, err
	}

	// 查找本地保存的认证信息
	authConfig, err := config.LookupAuthConfig(gatewayURL.String())
	if err != nil {
		fmt.Printf("Failed to lookup auth config: %s\n", err)
	}

	var clientAuth sdk.ClientAuth           // 客户端认证
	var functionTokenSource sdk.TokenSource // 函数调用令牌源

	// 处理 Basic 认证（来自 login 保存的用户名密码）
	if authConfig.Auth == config.BasicAuthType {
		username, password, err := config.DecodeAuth(authConfig.Token)
		if err != nil {
			return nil, err
		}

		clientAuth = &sdk.BasicAuth{
			Username: username,
			Password: password,
		}
	}

	// 处理 OAuth2 / Bearer 令牌认证
	if authConfig.Auth == config.Oauth2AuthType {
		tokenAuth := &StaticTokenAuth{
			token: authConfig.Token,
		}

		clientAuth = tokenAuth
		functionTokenSource = tokenAuth
	}

	// 命令行指定 --token 优先级最高
	if len(token) > 0 {
		tokenAuth := &StaticTokenAuth{
			token: token,
		}

		clientAuth = tokenAuth
		functionTokenSource = tokenAuth
	}

	// 创建 HTTP 客户端
	httpClient := &http.Client{}
	httpClient.Timeout = commandTimeout

	// 设置自定义传输（TLS/超时）
	transport := GetDefaultCLITransport(tlsInsecure, &commandTimeout)
	if transport != nil {
		httpClient.Transport = transport
	}

	// 构造并返回 SDK 客户端
	return sdk.NewClientWithOpts(gatewayURL, httpClient,
		sdk.WithAuthentication(clientAuth),
		sdk.WithFunctionTokenSource(functionTokenSource),
	), nil
}

// StaticTokenAuth 静态令牌认证实现
// 同时实现 sdk.ClientAuth 和 sdk.TokenSource 接口
type StaticTokenAuth struct {
	token string
}

// Set 向 HTTP 请求添加 Authorization: Bearer <token>
func (a *StaticTokenAuth) Set(req *http.Request) error {
	req.Header.Add("Authorization", "Bearer "+a.token)
	return nil
}

// Token 返回当前令牌（实现 TokenSource 接口）
func (ts *StaticTokenAuth) Token() (string, error) {
	return ts.token, nil
}
