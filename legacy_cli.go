// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"
)

// translateLegacyOpts
// 作用：将【旧版 Go 风格单横杠参数】翻译成【新版 CLI 双横杠标准参数】
// 例如：-handler → --handler
// 输入：原始命令行参数 args
// 输出：翻译后的新参数列表、错误信息
func translateLegacyOpts(args []string) ([]string, error) {

	// 旧参数 → 新参数 的映射表
	// 旧：单横杠 -xxx
	// 新：双横杠 --xxx
	legacyOptMapping := map[string]string{
		"-handler":  "--handler",
		"-image":    "--image",
		"-name":     "--name",
		"-gateway":  "--gateway",
		"-fprocess": "--fprocess",
		"-lang":     "--lang",
		"-replace":  "--replace",
		"-no-cache": "--no-cache",
		"-yaml":     "--yaml",
		"-squash":   "--squash",
	}

	// 合法的操作命令（旧版 -action 对应的新版操作）
	validActions := map[string]string{
		"build":  "build",
		"delete": "remove",
		"deploy": "deploy",
		"push":   "push",
	}

	action := ""                        // 最终要执行的动作（build/deploy/push/remove等）
	translatedArgs := []string{args[0]} // 翻译后的新参数，第一个是程序名，不变
	optsCache := args[1:]               // 除程序名外的所有参数

	// ===================== 第一步：处理旧版 -action / -version 参数 =====================
	// 遍历所有参数，替换旧版的 -action 和 -version
	for idx, opt := range optsCache {
		// 如果是旧版 -version，替换成 "version" 命令
		if opt == "-version" {
			translatedArgs = append(translatedArgs, "version")
			optsCache = append(optsCache[:idx], optsCache[idx+1:]...)
			action = "version"
		}

		// 如果是旧版 -action（空格形式：-action build）
		if opt == "-action" {
			// 检查后面有没有跟动作
			if len(optsCache) == idx+1 {
				return []string{""}, fmt.Errorf("no action supplied after deprecated -action flag")
			}
			// 如果动作合法，就翻译成新版动作
			if translated, ok := validActions[optsCache[idx+1]]; ok {
				translatedArgs = append(translatedArgs, translated)
				optsCache = append(optsCache[:idx], optsCache[idx+2:]...) // 删掉 -action 和它的值
				action = translated
			} else {
				return []string{""}, fmt.Errorf("unknown action supplied to deprecated -action flag: %s", optsCache[idx+1])
			}
		}

		// 如果是旧版 -action=xxx（等号形式：-action=build）
		if strings.HasPrefix(opt, "-action"+"=") {
			s := strings.SplitN(opt, "=", 2)
			if len(s[1]) == 0 {
				return []string{""}, fmt.Errorf("no action supplied after deprecated -action= flag")
			}
			// 合法则翻译
			if translated, ok := validActions[s[1]]; ok {
				translatedArgs = append(translatedArgs, translated)
				optsCache = append(optsCache[:idx], optsCache[idx+1:]...)
				action = translated
			} else {
				return []string{""}, fmt.Errorf("unknown action supplied to deprecated -action= flag: %s", s[1])
			}
		}
	}

	// ===================== 第二步：把所有单横杠 -xxx 换成双横杠 --xxx =====================
	for idx, arg := range optsCache {
		// 特殊处理：如果动作是删除(remove)，则去掉旧版的 -name
		if action == "remove" {
			if arg == "-name" {
				optsCache = append(optsCache[:idx], optsCache[idx+1:]...)
				continue
			}
		}

		// 精确匹配：直接替换
		if translated, ok := legacyOptMapping[arg]; ok {
			optsCache[idx] = translated
		}

		// 前缀匹配：处理 -name=test 这种带等号的形式
		for legacyOpt, translated := range legacyOptMapping {
			if strings.HasPrefix(arg, legacyOpt) {
				optsCache[idx] = strings.Replace(arg, legacyOpt, translated, 1)
			}
		}
	}

	// 把处理完的参数拼回结果
	translatedArgs = append(translatedArgs, optsCache...)

	// ===================== 第三步：如果有变化，打印提示用户 =====================
	if !reflect.DeepEqual(args, translatedArgs) {
		fmt.Fprintln(os.Stderr, "Found deprecated go-style flags in command, translating to new format:")
		fmt.Fprintf(os.Stderr, "  %s\n", strings.Join(translatedArgs, " "))
	}

	// 返回翻译后的参数
	return translatedArgs, nil
}
