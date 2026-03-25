// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package exec 提供系统命令执行工具函数
// 封装了 os/exec 标准库，提供带目录执行、输出返回、错误彩色打印等增强功能
package exec

import (
	"fmt"
	"log"
	"os"
	osexec "os/exec" // 系统标准exec包，避免包名冲突

	"github.com/morikuni/aec" // 终端彩色输出库
)

// Command 在指定目录下执行系统命令，直接输出 stdout/stderr 到终端
// 参数：
//
//	tempPath - 命令执行的工作目录
//	builder  - 命令切片，第一个元素为命令，后续为参数
//
// 说明：命令执行失败会输出红色错误信息并退出程序
func Command(tempPath string, builder []string) {
	// 构建系统命令
	targetCmd := osexec.Command(builder[0], builder[1:]...)
	// 设置命令执行目录
	targetCmd.Dir = tempPath
	// 将命令输出重定向到当前程序终端
	targetCmd.Stdout = os.Stdout
	targetCmd.Stderr = os.Stderr

	// 启动命令
	targetCmd.Start()
	// 等待命令执行完成
	err := targetCmd.Wait()
	if err != nil {
		errString := fmt.Sprintf("ERROR - Could not execute command: %s", builder)
		// 输出红色错误并退出
		log.Fatal(aec.RedF.Apply(errString))
	}
}

// CommandWithOutput 执行系统命令并返回合并后的输出结果
// 参数：
//
//	builder     - 命令切片，第一个元素为命令，后续为参数
//	skipFailure - 是否忽略命令执行错误，true 则不退出程序
//
// 返回：命令执行的 stdout + stderr 字符串
func CommandWithOutput(builder []string, skipFailure bool) string {
	// 执行命令并获取标准输出+标准错误合并结果
	output, err := osexec.Command(builder[0], builder[1:]...).CombinedOutput()
	// 命令执行失败且不忽略错误时，输出红色错误并退出
	if err != nil && !skipFailure {
		errString := fmt.Sprintf("ERROR - Could not execute command: %s", builder)
		log.Fatal(aec.RedF.Apply(errString))
	}

	return string(output)
}
