// Copyright (c) OpenFaaS Author(s) 2018. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 template store describe 命令，用于查看单个模板的详细信息
package commands

import (
	"bytes"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// init 初始化 template store describe 命令，注册命令行参数并添加到父命令
func init() {
	templateStoreDescribeCmd.PersistentFlags().StringVarP(&templateStoreURL, "url", "u", DefaultTemplatesStore, "Use as alternative store for templates")

	templateStoreCmd.AddCommand(templateStoreDescribeCmd)
}

// templateStoreDescribeCmd 查看模板仓库中单个函数模板的详细信息
var templateStoreDescribeCmd = &cobra.Command{
	Use:   `describe`,
	Short: `Describe the template`,
	Long:  `Describe the template by outputting all the fields that the template struct has`,
	Example: `  faas-cli template store describe golang-http
  faas-cli template store describe haskell --url https://raw.githubusercontent.com/custom/store/master/templates.json`,
	RunE: runTemplateStoreDescribe,
}

// runTemplateStoreDescribe 执行模板详情查询的核心逻辑
// 1. 校验命令行参数
// 2. 获取模板仓库地址与模板清单
// 3. 查找并验证指定模板是否存在
// 4. 格式化并输出模板详细信息
func runTemplateStoreDescribe(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("\nNeed to specify one of the store templates, check available ones by running the command:\n\nfaas-cli template store list\n")
	}
	if len(args) > 1 {
		return fmt.Errorf("\nNeed to specify single template from the store, check available ones by running the command:\n\nfaas-cli template store list\n")
	}
	envTemplateRepoStore := os.Getenv(templateStoreURLEnvironment)
	storeURL := getTemplateStoreURL(templateStoreURL, envTemplateRepoStore, DefaultTemplatesStore)

	templatesInfo, templatesErr := getTemplateInfo(storeURL)
	if templatesErr != nil {
		return fmt.Errorf("error while getting templates info: %s", templatesErr)
	}
	template := args[0]
	storeTemplate, templateErr := checkExistingTemplate(templatesInfo, template)
	if templateErr != nil {
		return fmt.Errorf("error while searching for template in store: %s", templateErr.Error())
	}

	templateInfo := formatTemplateOutput(storeTemplate)
	fmt.Fprintf(cmd.OutOrStdout(), "%s", templateInfo)

	return nil
}

// checkExistingTemplate 在模板列表中查找指定名称的模板
// 支持直接名称 或 源/模板名 两种格式匹配
func checkExistingTemplate(storeTemplates []TemplateInfo, template string) (TemplateInfo, error) {
	var existingTemplate TemplateInfo
	for _, storeTemplate := range storeTemplates {
		sourceName := fmt.Sprintf("%s/%s", storeTemplate.Source, storeTemplate.TemplateName)
		if template == storeTemplate.TemplateName || template == sourceName {
			existingTemplate = storeTemplate
			return existingTemplate, nil
		}
	}
	return existingTemplate, fmt.Errorf("template with name: `%s` does not exist in the store", template)
}

// formatTemplateOutput 将模板结构体格式化为美观的终端输出格式
// 使用 tabwriter 对齐字段，展示名称、描述、语言、仓库、平台、官方/推荐状态等
func formatTemplateOutput(storeTemplate TemplateInfo) string {
	var buff bytes.Buffer
	lineWriter := tabwriter.NewWriter(&buff, 0, 0, 1, ' ', 0)
	fmt.Fprintln(lineWriter)
	fmt.Fprintf(lineWriter, "Name:\t%s\n", storeTemplate.TemplateName)
	fmt.Fprintf(lineWriter, "Description:\t%s\n", storeTemplate.Description)

	fmt.Fprintf(lineWriter, "Language:\t%s\n", storeTemplate.Language)
	fmt.Fprintf(lineWriter, "Source:\t%s\n", storeTemplate.Source)
	fmt.Fprintf(lineWriter, "Repository:\t%s\n", storeTemplate.Repository)

	fmt.Fprintf(lineWriter, "Platform:\t%s\n", storeTemplate.Platform)

	if storeTemplate.Official == "true" {
		fmt.Fprintf(lineWriter, "Official:\t%s\n", "[x]")
	} else {
		fmt.Fprintf(lineWriter, "Official:\t%s\n", "[ ]")
	}

	if storeTemplate.Recommended {
		fmt.Fprintf(lineWriter, "Recommended:\t%s\n", "[x]")
	} else {
		fmt.Fprintf(lineWriter, "Recommended:\t%s\n", "[ ]")
	}

	fmt.Fprintln(lineWriter)

	lineWriter.Flush()

	return buff.String()
}
