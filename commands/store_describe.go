// Copyright (c) OpenFaaS Author(s) 2018. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 store describe 命令，用于查看商店中单个函数的详细信息
package commands

import (
	"bytes"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mitchellh/go-wordwrap"
	storeV2 "github.com/openfaas/faas-cli/schema/store/v2"
	"github.com/spf13/cobra"
)

// init 初始化 store describe 命令并注册到 store 根命令
func init() {
	storeCmd.AddCommand(storeDescribeCmd)
}

// storeDescribeCmd 查看商店中函数的详细信息
// 支持别名 inspect，可通过函数名或标题查询
var storeDescribeCmd = &cobra.Command{
	Use:   `describe (FUNCTION_NAME|FUNCTION_TITLE) [--url STORE_URL]`,
	Short: "Show details of OpenFaaS function from a store",
	Example: `  faas-cli store describe nodeinfo
  faas-cli store describe nodeinfo --url https://host:port/store.json
`,
	Aliases: []string{"inspect"},
	RunE:    runStoreDescribe,
}

// runStoreDescribe 执行函数详情查询的核心逻辑
// 1. 校验参数
// 2. 获取目标平台
// 3. 读取商店地址（环境变量/命令行）
// 4. 获取并过滤函数列表
// 5. 查找并渲染函数详情
func runStoreDescribe(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("please provide the function name")
	}

	targetPlatform := getTargetPlatform(platformValue)

	// 环境变量 OPENFAAS_STORE 优先级：未手动指定 --url 时生效
	if v, ok := os.LookupEnv("OPENFAAS_STORE"); ok && len(v) > 0 {
		if !storeCmd.Flags().Changed("url") {
			storeAddress = v
		}
	}

	storeItems, err := storeList(storeAddress)
	if err != nil {
		return err
	}

	platformFunctions := filterStoreList(storeItems, targetPlatform)

	functionName := args[0]
	item := storeFindFunction(functionName, platformFunctions)
	if item == nil {
		return fmt.Errorf("function '%s' not found for platform '%s'", functionName, targetPlatform)
	}

	content := storeRenderItem(item, targetPlatform)
	fmt.Print(content)

	return nil
}

// storeRenderItem 将函数信息格式化为美观的终端输出格式
// 包含标题、作者、描述、镜像、环境变量、标签、注解等字段
func storeRenderItem(item *storeV2.StoreFunction, platform string) string {
	var b bytes.Buffer
	w := tabwriter.NewWriter(&b, 0, 0, 1, ' ', 0)

	author := item.Author
	if author == "" {
		author = "unknown"
	}

	fmt.Fprintf(w, "%s\t%s\n", "Title:", item.Title)
	fmt.Fprintf(w, "%s\t%s\n", "Author:", item.Author)
	fmt.Fprintf(w, "%s\t\n%s\n\n", "Description:", wordwrap.WrapString(item.Description, 80))

	fmt.Fprintf(w, "%s\t%s\n", "Image:", item.GetImageName(platform))
	fmt.Fprintf(w, "%s\t%s\n", "Process:", item.Fprocess)
	fmt.Fprintf(w, "%s\t%s\n", "Repo URL:", item.RepoURL)

	// 输出环境变量
	if len(item.Environment) > 0 {
		fmt.Fprintf(w, "Environment:\n")
		for k, v := range item.Environment {
			fmt.Fprintf(w, "- \t%s:\t%s\n", k, v)
		}
		fmt.Fprintln(w)
	}

	// 输出标签
	if item.Labels != nil {
		fmt.Fprintf(w, "Labels:\n")
		for k, v := range item.Labels {
			fmt.Fprintf(w, "- \t%s:\t%s\n", k, v)
		}
		fmt.Fprintln(w)
	}

	// 输出注解
	if len(item.Annotations) > 0 {
		fmt.Fprintf(w, "Annotations:\n")
		for k, v := range item.Annotations {
			fmt.Fprintf(w, "- \t%s:\t%s\n", k, v)
		}
		fmt.Fprintln(w)
	}

	w.Flush()
	return b.String()
}
