// Copyright (c) OpenFaaS Author(s) 2018. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 template store list 命令，用于列出模板仓库中的可用函数模板
package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

const (
	// DefaultTemplatesStore 官方模板仓库的默认地址
	DefaultTemplatesStore = "https://raw.githubusercontent.com/openfaas/store/master/templates.json"
	// mainPlatform 默认展示的平台架构
	mainPlatform = "x86_64"
)

// 命令行全局参数
var (
	templateStoreURL string // 模板仓库URL
	inputPlatform    string // 过滤模板的平台架构
	recommended      bool   // 仅展示推荐模板
	official         bool   // 仅展示官方模板
)

// init 初始化 template store list 命令，注册命令行标志并添加到父命令
func init() {
	templateStoreListCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Shows additional language and platform")
	templateStoreListCmd.PersistentFlags().StringVarP(&templateStoreURL, "url", "u", DefaultTemplatesStore, "Use as alternative store for templates")
	templateStoreListCmd.Flags().StringVarP(&inputPlatform, "platform", "p", mainPlatform, "Shows the platform if the output is verbose")
	templateStoreListCmd.Flags().BoolVarP(&recommended, "recommended", "r", false, "Shows only recommended templates")
	templateStoreListCmd.Flags().BoolVarP(&official, "official", "o", false, "Shows only official templates")

	templateStoreCmd.AddCommand(templateStoreListCmd)
}

// templateStoreListCmd 列出默认/自定义模板仓库中的可用函数模板
var templateStoreListCmd = &cobra.Command{
	Use:     `list`,
	Short:   `List templates from OpenFaaS organizations`,
	Aliases: []string{"ls"},
	Long: `List templates from a template store manifest file, by default the 
official list maintained by the OpenFaaS community is used. You can override this.`,
	Example: `  faas-cli template store list
  # List only recommended templates
  faas-cli template store list --recommended

  # List only official templates
  faas-cli template store list --official

  # Override the store via a flag
  faas-cli template store ls \
  --url=https://raw.githubusercontent.com/openfaas/store/master/templates.json

  # Specify an alternative store via environment variable
  export OPENFAAS_TEMPLATE_STORE_URL=https://example.com/templates.json

  # See additional language and platform
  faas-cli template store ls --verbose=true

  # Filter by platform for arm64 only
  faas-cli template store list --platform arm64 
`,
	RunE: runTemplateStoreList,
}

// runTemplateStoreList 执行模板列表命令的核心逻辑
// 读取仓库配置、获取模板清单、过滤并格式化输出结果
func runTemplateStoreList(cmd *cobra.Command, args []string) error {
	envTemplateRepoStore := os.Getenv(templateStoreURLEnvironment)
	storeURL := getTemplateStoreURL(templateStoreURL, envTemplateRepoStore, DefaultTemplatesStore)

	templatesInfo, err := getTemplateInfo(storeURL)
	if err != nil {
		return fmt.Errorf("error while getting templates info: %s", err)
	}
	list := []TemplateInfo{}

	// 根据参数过滤模板：推荐/官方/全部
	if recommended {
		for i := 0; i < len(templatesInfo); i++ {
			if templatesInfo[i].Recommended {
				list = append(list, templatesInfo[i])
			}
		}
	} else if official {
		for i := 0; i < len(templatesInfo); i++ {
			if templatesInfo[i].Official == "true" {
				list = append(list, templatesInfo[i])
			}
		}
	} else {
		list = templatesInfo
	}

	formattedOutput := formatTemplatesOutput(list, verbose, inputPlatform)

	fmt.Fprintf(cmd.OutOrStdout(), "%s", formattedOutput)

	return nil
}

// getTemplateInfo 通过HTTP请求获取模板仓库的模板清单
// 解析JSON并返回结构化的模板信息列表
func getTemplateInfo(repository string) ([]TemplateInfo, error) {
	req, reqErr := http.NewRequest(http.MethodGet, repository, nil)
	if reqErr != nil {
		return nil, fmt.Errorf("error while trying to create request to take template info: %s", reqErr.Error())
	}

	reqContext, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()
	req = req.WithContext(reqContext)

	client := http.DefaultClient
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error while requesting template list: %s", err.Error())
	}

	if res.Body == nil {
		return nil, fmt.Errorf("error empty response body from: %s", templateStoreURL)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, res.Body) // 排空响应体
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code wanted: %d got: %d", http.StatusOK, res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error while reading response: %s", err.Error())
	}

	templatesInfo := []TemplateInfo{}
	if err := json.Unmarshal(body, &templatesInfo); err != nil {
		return nil, fmt.Errorf("can't unmarshal text: %s, value: %s", err.Error(), string(body))
	}

	sortTemplates(templatesInfo)

	return templatesInfo, nil
}

// sortTemplates 对模板列表进行排序
// 排序规则：推荐模板 > 官方模板 > 模板名称(字母序)
func sortTemplates(templatesInfo []TemplateInfo) {
	sort.Slice(templatesInfo, func(i, j int) bool {
		if templatesInfo[i].Recommended == templatesInfo[j].Recommended {
			if templatesInfo[i].Official == templatesInfo[j].Official {
				return strings.ToLower(templatesInfo[i].TemplateName) < strings.ToLower(templatesInfo[j].TemplateName)
			} else {
				return templatesInfo[i].Official < templatesInfo[j].Official
			}
		} else if templatesInfo[i].Recommended {
			return true
		} else {
			return false
		}
	})
}

// formatTemplatesOutput 格式化模板列表为终端友好输出
// 支持按平台过滤、普通/详细两种展示模式
func formatTemplatesOutput(templates []TemplateInfo, verbose bool, platform string) string {

	if platform != mainPlatform {
		templates = filterTemplate(templates, platform)
	} else {
		templates = filterTemplate(templates, mainPlatform)
	}

	if len(templates) == 0 {
		return ""
	}

	var buff bytes.Buffer
	lineWriter := tabwriter.NewWriter(&buff, 0, 0, 1, ' ', 0)

	fmt.Fprintln(lineWriter)
	if verbose {
		formatVerboseOutput(lineWriter, templates)
	} else {
		formatBasicOutput(lineWriter, templates)
	}
	fmt.Fprintln(lineWriter)

	lineWriter.Flush()

	return buff.String()
}

// formatBasicOutput 输出基础版模板列表（简洁模式）
func formatBasicOutput(lineWriter *tabwriter.Writer, templates []TemplateInfo) {

	fmt.Fprintf(lineWriter, "NAME\tRECOMMENDED\tDESCRIPTION\tSOURCE\n")
	for _, template := range templates {

		recommended := "[ ]"
		if template.Recommended {
			recommended = "[x]"
		}

		fmt.Fprintf(lineWriter, "%s\t%s\t%s\t%s\n",
			template.TemplateName,
			recommended,
			template.Source,
			template.Description)
	}
}

// formatVerboseOutput 输出详细版模板列表（verbose模式）
func formatVerboseOutput(lineWriter *tabwriter.Writer, templates []TemplateInfo) {

	fmt.Fprintf(lineWriter, "NAME\tRECOMMENDED\tSOURCE\tDESCRIPTION\tLANGUAGE\tPLATFORM\n")
	for _, template := range templates {
		recommended := "[ ]"
		if template.Recommended {
			recommended = "[x]"
		}

		fmt.Fprintf(lineWriter, "%s\t%s\t%s\t%s\t%s\t%s\n",
			template.TemplateName,
			recommended,
			template.Source,
			template.Description,
			template.Language,
			template.Platform)
	}
}

// TemplateInfo 模板仓库中单个函数模板的结构体定义
type TemplateInfo struct {
	TemplateName string `json:"template"`
	Platform     string `json:"platform"`
	Language     string `json:"language"`
	Source       string `json:"source"`
	Description  string `json:"description"`
	Repository   string `json:"repo"`
	Official     string `json:"official"`
	Recommended  bool   `json:"recommended"`
}

// filterTemplate 根据平台架构过滤模板列表
func filterTemplate(templates []TemplateInfo, platform string) []TemplateInfo {
	var filteredTemplates []TemplateInfo

	for _, template := range templates {
		if strings.EqualFold(template.Platform, platform) {
			filteredTemplates = append(filteredTemplates, template)
		}
	}
	return filteredTemplates
}
