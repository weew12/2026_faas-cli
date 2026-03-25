// Copyright (c) OpenFaaS Author(s) 2018. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 store list 命令，用于列出函数商店中可用的函数
package commands

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	storeV2 "github.com/openfaas/faas-cli/schema/store/v2"
	"github.com/spf13/cobra"
)

// init 初始化 store list 命令，注册命令行参数并添加到 store 根命令
func init() {
	// Setup flags used by store command
	storeListCmd.Flags().BoolVarP(&verbose, "verbose", "v", true, "Enable verbose output to see the full description of each function in the store")

	storeCmd.AddCommand(storeListCmd)
}

// storeListCmd 列出函数商店中的可用函数
// 支持别名 ls，可指定商店地址、开启详细描述
var storeListCmd = &cobra.Command{
	Use:     `list [--url STORE_URL]`,
	Aliases: []string{"ls"},
	Short:   "List available OpenFaaS functions in a store",
	Example: `  faas-cli store list
  faas-cli store list --verbose
  faas-cli store list --url https://host:port/store.json`,
	RunE: runStoreList,
}

// runStoreList 执行 store list 命令的核心逻辑
// 1. 获取目标平台
// 2. 读取环境变量或命令行指定的商店地址
// 3. 从商店获取函数列表并按平台过滤
// 4. 格式化输出结果
func runStoreList(cmd *cobra.Command, args []string) error {
	targetPlatform := getTargetPlatform(platformValue)

	// Support priority order override.
	// --url by default, unless OPENFAAS_STORE is given, in which
	// case the flag takes precedence if it has been changed from its
	// default
	if v, ok := os.LookupEnv("OPENFAAS_STORE"); ok && len(v) > 0 {
		if !storeCmd.Flags().Changed("url") {
			storeAddress = v
		}
	}

	storeList, err := storeList(storeAddress)
	if err != nil {
		return err
	}

	filteredFunctions := filterStoreList(storeList, targetPlatform)

	if len(filteredFunctions) == 0 {
		availablePlatforms := getStorePlatforms(storeList)
		fmt.Printf("No functions found in the store for platform '%s', try one of the following: %s\n", targetPlatform, strings.Join(availablePlatforms, ", "))
		return nil
	}

	fmt.Print(storeRenderItems(filteredFunctions))

	return nil
}

// storeRenderItems 将函数列表格式化为表格形式输出
// 使用 tabwriter 对齐列，展示函数名、作者、描述
func storeRenderItems(items []storeV2.StoreFunction) string {
	var b bytes.Buffer
	w := tabwriter.NewWriter(&b, 0, 0, 1, ' ', 0)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "FUNCTION\tAUTHOR\tDESCRIPTION")

	for _, item := range items {
		author := item.Author
		if author == "" {
			author = "unknown"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\n", item.Name, author, storeRenderDescription(item.Title))
	}

	fmt.Fprintln(w)
	w.Flush()
	return b.String()
}

// storeRenderDescription 格式化函数描述
// 非 verbose 模式下超长描述自动截断并添加 ...
func storeRenderDescription(descr string) string {
	if !verbose && len(descr) > maxDescriptionLen {
		return descr[0:maxDescriptionLen-3] + "..."
	}

	return descr
}
