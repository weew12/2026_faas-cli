// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件提供**公共工具函数**，用于配置解析、环境变量处理、命名空间/地址计算等
package commands

import (
	"fmt"
	"os"
	"strings"
)

// 环境变量常量定义
const (
	openFaaSURLEnvironment      = "OPENFAAS_URL"                // OpenFaaS 网关地址环境变量
	remoteBuilderEnvironment    = "OPENFAAS_REMOTE_BUILDER"     // 远程构建器地址环境变量
	payloadSecretEnvironment    = "OPENFAAS_PAYLOAD_SECRET"     // 负载密钥环境变量
	builderPublicKeyEnvironment = "OPENFAAS_BUILDER_PUBLIC_KEY" // 构建器公钥环境变量
	templateURLEnvironment      = "OPENFAAS_TEMPLATE_URL"       // 模板地址环境变量
	templateStoreURLEnvironment = "OPENFAAS_TEMPLATE_STORE_URL" // 模板仓库地址环境变量
	defaultFunctionNamespace    = ""                            // 默认命名空间（空字符串）
)

// getGatewayURL 按优先级获取最终网关地址
// 优先级：命令行参数 > stack.yaml 配置 > 环境变量 > 默认值
// 自动补全 http:// 前缀并统一格式
func getGatewayURL(argumentURL, defaultURL, yamlURL, environmentURL string) string {
	var gatewayURL string

	// 按优先级选择地址
	if len(argumentURL) > 0 && argumentURL != defaultURL {
		gatewayURL = argumentURL
	} else if len(yamlURL) > 0 && yamlURL != defaultURL {
		gatewayURL = yamlURL
	} else if len(environmentURL) > 0 {
		gatewayURL = environmentURL
	} else {
		gatewayURL = defaultURL
	}

	// 格式化：小写 + 去除末尾 /
	gatewayURL = strings.ToLower(strings.TrimRight(gatewayURL, "/"))
	// 自动补全 HTTP 前缀
	if !strings.HasPrefix(gatewayURL, "http") {
		gatewayURL = fmt.Sprintf("http://%s", gatewayURL)
	}

	return gatewayURL
}

// getTemplateURL 按优先级获取模板仓库地址
// 优先级：命令行参数 > 环境变量 > 默认值
func getTemplateURL(argumentURL, environmentURL, defaultURL string) string {
	var templateURL string

	if len(argumentURL) > 0 && argumentURL != defaultURL {
		templateURL = argumentURL
	} else if len(environmentURL) > 0 {
		templateURL = environmentURL
	} else {
		templateURL = defaultURL
	}

	return templateURL
}

// getTemplateStoreURL 按优先级获取模板商店地址
// 优先级：命令行参数 > 环境变量 > 默认值
func getTemplateStoreURL(argumentURL, environmentURL, defaultURL string) string {
	if argumentURL != defaultURL {
		return argumentURL
	} else if len(environmentURL) > 0 {
		return environmentURL
	} else {
		return defaultURL
	}
}

// getNamespace 按优先级获取函数命名空间
// 优先级：命令行 --namespace > stack.yaml 配置 > 默认空值
func getNamespace(flagNamespace, stackNamespace string) string {
	// 命令行传入命名空间优先
	if len(flagNamespace) > 0 {
		return flagNamespace
	}
	// 其次使用 stack.yaml 中的配置
	if len(stackNamespace) > 0 {
		return stackNamespace
	}

	// 无配置时返回默认空命名空间
	return defaultFunctionNamespace
}

// getStringValue 获取字符串值：优先使用命令行值，否则使用环境变量
func getStringValue(flagValue, environmentValue string) string {
	if len(flagValue) > 0 {
		return flagValue
	}

	return environmentValue
}

// applyRemoteBuilderEnvironment 加载远程构建器相关环境变量
// 覆盖 remoteBuilder / payloadSecretPath / builderPublicKeyPath 全局变量
func applyRemoteBuilderEnvironment() {
	remoteBuilder = getStringValue(remoteBuilder, os.Getenv(remoteBuilderEnvironment))
	payloadSecretPath = getStringValue(payloadSecretPath, os.Getenv(payloadSecretEnvironment))
	builderPublicKeyPath = getStringValue(builderPublicKeyPath, os.Getenv(builderPublicKeyEnvironment))
}
