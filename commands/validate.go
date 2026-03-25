// Copyright (c) OpenFaaS Author(s) 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件提供语言参数校验与标准化处理功能
package commands

import (
	"errors"
)

// validateLanguageFlag 校验并标准化语言模板参数
// 将大写的 "Dockerfile" 统一转换为小写 "dockerfile"，保证参数格式一致
// 参数：
//
//	language - 命令行传入的语言模板字符串
//
// 返回：
//
//	string - 标准化后的语言模板
//	error  - 转换时的提示信息（非严重错误）
func validateLanguageFlag(language string) (string, error) {
	var err error

	// 统一将大写 Dockerfile 转换为小写格式
	if language == "Dockerfile" {
		language = "dockerfile"
		// 返回提示信息告知用户已自动修正参数
		err = errors.New(`language "Dockerfile" was converted to "dockerfile" automatically`)
	}

	return language, err
}
