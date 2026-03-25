// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 template pull stack 命令，从 stack.yml 自动拉取所有依赖的函数模板
package commands

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/openfaas/go-sdk/stack"
	"github.com/spf13/cobra"
)

// 全局命令行参数
var (
	templateURL    string // 模板仓库URL
	customRepoName string // 自定义仓库名称
)

// init 初始化 template pull stack 命令，注册参数并添加到父命令
func init() {
	templatePullStackCmd.Flags().BoolVar(&overwrite, "overwrite", true, "Overwrite existing templates?")
	templatePullStackCmd.Flags().BoolVar(&pullDebug, "debug", false, "Enable debug output")

	templatePullCmd.AddCommand(templatePullStackCmd)
}

// templatePullStackCmd 从函数 YAML 定义文件中读取并下载所需模板
var templatePullStackCmd = &cobra.Command{
	Use:   `stack`,
	Short: `Downloads templates specified in the function definition yaml file`,
	Long: `Downloads templates specified in the function yaml file, in the current directory
	`,
	Example: `
  faas-cli template pull stack
  faas-cli template pull stack -f myfunction.yml
  faas-cli template pull stack -r custom_repo_name
`,
	RunE: runTemplatePullStack,
}

// runTemplatePullStack 执行拉取 stack.yml 配置模板的入口函数
func runTemplatePullStack(cmd *cobra.Command, args []string) error {
	templatesConfig, err := loadTemplateConfig()
	if err != nil {
		return err
	}

	return pullConfigTemplates(templatesConfig)
}

// loadTemplateConfig 加载 stack.yml 中的模板仓库配置
func loadTemplateConfig() ([]stack.TemplateSource, error) {
	stackConfig, err := readStackConfig()
	if err != nil {
		return nil, err
	}
	return stackConfig.StackConfig.TemplateConfigs, nil
}

// readStackConfig 读取并解析 stack.yml 配置文件
func readStackConfig() (stack.Configuration, error) {
	configField := stack.Configuration{}

	configFieldBytes, err := os.ReadFile(yamlFile)
	if err != nil {
		return configField, fmt.Errorf("can't read file %s, error: %s", yamlFile, err.Error())
	}
	if err := yaml.Unmarshal(configFieldBytes, &configField); err != nil {
		return configField, fmt.Errorf("can't read: %s", err.Error())
	}

	if len(configField.StackConfig.TemplateConfigs) == 0 {
		return configField, fmt.Errorf("can't read configuration: no template repos currently configured")
	}
	return configField, nil
}

// pullConfigTemplates 批量拉取配置文件中的所有模板
func pullConfigTemplates(templateSources []stack.TemplateSource) error {
	for _, config := range templateSources {
		fmt.Printf("Pulling template: %s from %s\n", config.Name, config.Source)

		if err := pullTemplate(config.Source, config.Name, overwrite); err != nil {
			return err
		}
	}
	return nil
}

// pullStackTemplates 根据缺失的模板列表，从仓库或官方模板库拉取模板
func pullStackTemplates(missingTemplates []string, templateSources []stack.TemplateSource, cmd *cobra.Command) error {

	for _, val := range missingTemplates {

		var templateConfig stack.TemplateSource
		for _, config := range templateSources {
			if config.Name == val {
				templateConfig = config
				break
			}
		}

		if templateConfig.Source == "" {
			fmt.Printf("Pulling template: %s from store\n", val)

			if err := runTemplateStorePull(cmd, []string{val}); err != nil {
				return err
			}
		} else {
			fmt.Printf("Pulling template: %s from %s\n", val, templateConfig.Source)

			templateName := templateConfig.Name
			if err := pullTemplate(templateConfig.Source, templateName, overwrite); err != nil {
				return err
			}
		}
	}

	return nil
}

// getMissingTemplates 过滤本地已存在的模板，返回缺失的模板列表
func getMissingTemplates(functions map[string]stack.Function, templatesDir string) ([]string, error) {
	var missing []string

	for _, function := range functions {
		templatePath := fmt.Sprintf("%s/%s", templatesDir, function.Language)
		if _, err := os.Stat(templatePath); err != nil && os.IsNotExist(err) {
			missing = append(missing, function.Language)
		}
	}

	return missing, nil
}
