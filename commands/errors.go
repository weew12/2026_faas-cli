// Copyright (c) OpenFaaS Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"strings"
)

// 常量定义
const (
	// NoTLSWarn 当网关未使用加密连接时显示的警告信息
	NoTLSWarn = "WARNING! You are not using an encrypted connection to the gateway, consider using HTTPS."
)

// checkTLSWarnInsecure
// 检查网关地址是否使用不安全的明文 HTTP 连接
// 如果满足以下条件，返回安全警告：
// 1. 未使用 --tls-no-verify 跳过验证
// 2. 地址不是 https 开头
// 3. 地址不是本地回环地址（127.0.0.1 / localhost）
func checkTLSInsecure(gateway string, tlsInsecure bool) string {

	// 如果用户没有指定跳过 TLS 验证，则进行安全检查
	if !tlsInsecure {

		// 条件：
		// 1. 不是 https 协议
		// 2. 不是 http://127.0.0.1
		// 3. 不是 http://localhost
		if strings.HasPrefix(gateway, "https") == false &&
			strings.HasPrefix(gateway, "http://127.0.0.1") == false &&
			strings.HasPrefix(gateway, "http://localhost") == false {

			// 返回不安全连接警告
			return NoTLSWarn
		}
	}

	// 安全则返回空字符串
	return ""
}
