// Copyright (c) OpenFaaS Author(s) 2018. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 template store pull 命令，用于从官方模板仓库拉取函数模板
package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// init 初始化命令参数，注册 template store pull 子命令
func init() {
	// 绑定模板仓库 URL 参数
	templateStorePullCmd.PersistentFlags().StringVarP(&templateStoreURL, "url", "u", DefaultTemplatesStore, "Use as alternative store for templates")
	// 继承 template pull 命令的参数集合
	templatePull, _, _ := faasCmd.Find([]string{"template", "pull"})
	templateStoreCmd.PersistentFlags().AddFlagSet(templatePull.Flags())

	// 将 pull 命令添加为 template store 的子命令
	templateStoreCmd.AddCommand(templateStorePullCmd)
}

// templateStorePullCmd 从模板仓库拉取指定函数模板
// 支持官方仓库、自定义仓库、指定版本标签的模板拉取
var templateStorePullCmd = &cobra.Command{
	Use:   `pull [TEMPLATE_NAME]`,
	Short: `Pull templates from store`,
	Long:  `Pull templates from store supported by openfaas or openfaas-incubator organizations or your custom store`,
	Example: `  faas-cli template store pull ruby-http
  faas-cli template store pull go --debug
  faas-cli template store pull openfaas/go --overwrite
  faas-cli template store pull golang-middleware --url https://raw.githubusercontent.com/openfaas/store/master/templates.json`,
	RunE: runTemplateStorePull,
}

// runTemplateStorePull 执行模板拉取逻辑
// 1. 校验命令行参数
// 2. 获取模板仓库地址并读取模板清单
// 3. 匹配用户指定的模板
// 4. 调用底层模板拉取命令完成下载
func runTemplateStorePull(cmd *cobra.Command, args []string) error {
	// 必须指定模板名称
	if len(args) == 0 {
		return fmt.Errorf("need to specify one of the store templates, check available ones by running the command:\n\nfaas-cli template store list")
	}
	// 仅支持一次拉取一个模板
	if len(args) > 1 {
		return fmt.Errorf("need to specify single template from the store, check available ones by running the command:\n\nfaas-cli template store list")
	}

	// 确定最终使用的模板仓库地址（优先级：命令行参数 > 环境变量 > 默认值）
	envTemplateRepoStore := os.Getenv(templateStoreURLEnvironment)
	storeURL := getTemplateStoreURL(templateStoreURL, envTemplateRepoStore, DefaultTemplatesStore)

	// 从仓库获取模板清单
	storeTemplates, err := getTemplateInfo(storeURL)
	if err != nil {
		return fmt.Errorf("error while fetching templates from store: %s", err)
	}

	templateName := args[0]
	found := false

	// 遍历模板清单，匹配用户指定的模板
	for _, storeTemplate := range storeTemplates {
		// 切割模板名称与版本标签（如 go@1.2.3）
		_, ref, _ := strings.Cut(templateName, "@")

		// 构造源名称格式：源组织/模板名
		sourceName := fmt.Sprintf("%s/%s", storeTemplate.Source, storeTemplate.TemplateName)

		// 支持三种匹配格式：
		// 1. 纯模板名：go
		// 2. 带版本：go@1.2.3
		// 3. 完整源名：openfaas/go
		if templateName == storeTemplate.TemplateName ||
			(len(ref) > 0 && templateName == storeTemplate.TemplateName+"@"+ref) ||
			templateName == sourceName {

			// 获取模板仓库地址，若指定版本则添加 # 后缀
			repository := storeTemplate.Repository
			if ref != "" {
				repository = repository + "#" + ref
			}

			// 调用底层 template pull 命令拉取模板
			if err := runTemplatePull(cmd, []string{repository}); err != nil {
				return fmt.Errorf("error while pulling template: %s : %s", storeTemplate.TemplateName, err.Error())
			}

			found = true
			break
		}
	}

	// 未找到匹配的模板
	if !found {
		return fmt.Errorf("template with name: `%s` does not exist in the repo", templateName)
	}
	return nil
}
