// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

/*
Package main 是 faas-cli 命令行工具的入口包
该包负责解析命令行参数、处理兼容旧版选项，并启动 CLI 命令执行流程。
*/
package main

import (
	"fmt"
	"os"

	"github.com/openfaas/faas-cli/commands"
)

// main 是程序的主入口函数
// 功能：转换兼容旧版命令行参数，处理转换错误，最终调用命令执行器
func main() {
	// 转换旧版命令行参数为新版格式
	customArgs, err := translateLegacyOpts(os.Args)
	if err != nil {
		// 参数转换失败，输出错误信息并退出程序
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	// 执行 CLI 核心命令逻辑
	commands.Execute(customArgs)
}
