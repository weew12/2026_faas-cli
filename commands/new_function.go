// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 new 命令：用于**创建新函数**、生成目录、代码模板和 stack.yaml 配置
package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/openfaas/faas-cli/builder"
	"github.com/openfaas/go-sdk/stack"
	"github.com/spf13/cobra"
)

// 命令行标志变量
var (
	appendFile    string // 追加到已有的 YAML 文件
	list          bool   // 列出可用模板语言
	quiet         bool   // 安静模式，不显示模板说明信息
	memoryLimit   string // 内存限制
	cpuLimit      string // CPU 限制
	memoryRequest string // 内存请求
	cpuRequest    string // CPU 请求
)

// init 初始化 new 命令的参数并注册到主命令
func init() {
	newFunctionCmd.Flags().StringVar(&language, "lang", "", "Language or template to use")
	newFunctionCmd.Flags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL to store in YAML stack file")
	newFunctionCmd.Flags().StringVar(&handlerDir, "handler", "", "directory the handler will be written to")
	newFunctionCmd.Flags().StringVarP(&imagePrefix, "prefix", "p", "", "Set prefix for the function image")

	newFunctionCmd.Flags().StringVar(&memoryLimit, "memory-limit", "", "Set a limit for the memory")
	newFunctionCmd.Flags().StringVar(&cpuLimit, "cpu-limit", "", "Set a limit for the CPU")

	newFunctionCmd.Flags().StringVar(&memoryRequest, "memory-request", "", "Set a request or the memory")
	newFunctionCmd.Flags().StringVar(&cpuRequest, "cpu-request", "", "Set a request value for the CPU")

	newFunctionCmd.Flags().BoolVar(&list, "list", false, "List available languages")
	newFunctionCmd.Flags().StringVarP(&appendFile, "append", "a", "", "Append to existing YAML file")
	newFunctionCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Skip template notes")

	faasCmd.AddCommand(newFunctionCmd)
}

// newFunctionCmd 创建新函数的主命令
var newFunctionCmd = &cobra.Command{
	Use:   "new FUNCTION_NAME --lang=FUNCTION_LANGUAGE [--gateway=http://host:port] | --list | --append=STACK_FILE)",
	Short: "Create a new template in the current folder with the name given as name",
	Long: `The new command creates a new function based upon hello-world in the given
language or type in --list for a list of languages available.`,
	Example: `  faas-cli new chatbot --lang node
  faas-cli new chatbot --lang node --append bots.yaml
  faas-cli new text-parser --lang python --quiet
  faas-cli new text-parser --lang python --gateway http://mydomain:8080
  faas-cli new --list`,
	PreRunE: preRunNewFunction,
	RunE:    runNewFunction,
}

// validateFunctionName 验证函数名是否符合 Kubernetes DNS-1123 规范（仅小写字母、数字、横杠）
func validateFunctionName(functionName string) error {
	// Regex for RFC-1123 validation:
	// 	k8s.io/kubernetes/pkg/util/validation/validation.go
	var validDNS = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if matched := validDNS.MatchString(functionName); !matched {
		return fmt.Errorf(`function name can only contain a-z, 0-9 and dashes`)
	}
	return nil
}

// preRunNewFunction 命令执行前的参数校验
func preRunNewFunction(cmd *cobra.Command, args []string) error {
	// 如果是 --list，直接列出模板，无需校验
	if list {
		return nil
	}

	language, _ = validateLanguageFlag(language)

	// 未指定语言和函数名，显示帮助
	if len(language) == 0 && len(args) < 1 {
		cmd.Help()
		os.Exit(0)
	}
	// 必须指定 --lang
	if len(language) == 0 {
		return fmt.Errorf("you must supply a function language with the --lang flag")
	}

	// 必须指定函数名
	if len(args) < 1 {
		return fmt.Errorf(`please provide a name for the function`)
	}

	functionName = args[0]

	// 校验函数名格式
	if err := validateFunctionName(functionName); err != nil {
		return err
	}

	// 未指定 YAML 文件时使用默认值
	if len(yamlFile) == 0 && len(appendFile) == 0 {
		yamlFile = defaultYAML
	}

	return nil
}

// runNewFunction 执行创建新函数的核心逻辑
func runNewFunction(cmd *cobra.Command, args []string) error {
	// 1. --list 模式：列出所有可用模板
	if list {
		var availableTemplates []string

		templateFolders, err := os.ReadDir(TemplateDirectory)
		if err != nil {
			return fmt.Errorf(`no language templates were found.

Download templates:
  faas-cli template pull                download the default templates
  faas-cli template store list          view the template store
  faas-cli template store pull NAME     download the template from store
  faas-cli new --lang NAME              auto-download NAME from template store`)
		}

		for _, file := range templateFolders {
			if file.IsDir() {
				availableTemplates = append(availableTemplates, file.Name())
			}
		}

		fmt.Printf("Languages available as templates:\n%s\n", printAvailableTemplates(availableTemplates))

		return nil
	}

	// 2. 如果模板不存在，自动从模板仓库拉取
	if !stack.IsValidTemplate(language) {

		envTemplateRepoStore := os.Getenv(templateStoreURLEnvironment)
		storeURL := getTemplateStoreURL(templateStoreURL, envTemplateRepoStore, DefaultTemplatesStore)

		templatesInfo, err := getTemplateInfo(storeURL)
		if err != nil {
			return fmt.Errorf("error while getting templates info: %s", err)
		}

		_, ref, _ := strings.Cut(language, "@")

		var templateInfo *TemplateInfo
		for _, info := range templatesInfo {
			if info.TemplateName == language || (info.TemplateName+"@"+ref == language) {
				templateInfo = &info
				break
			}
		}

		if templateInfo == nil {
			return fmt.Errorf("template: \"%s\" was not found in the templates folder or in the store", language)
		}

		templateName := templateInfo.TemplateName
		if ref != "" {
			templateName += "#" + ref
		}

		if err := pullTemplate(templateInfo.Repository, templateName, overwrite); err != nil {
			return fmt.Errorf("error while pulling template: %s", err)
		}

	}

	// 3. 处理输出模式：新建 YAML 或追加到已有文件
	var fileName, outputMsg string
	appendMode := len(appendFile) > 0

	if appendMode {
		// 校验文件后缀
		if !(strings.HasSuffix(appendFile, ".yml") || strings.HasSuffix(appendFile, ".yaml")) {
			return fmt.Errorf("when appending to a stack the suffix should be .yml or .yaml")
		}

		// 校验文件是否存在
		if _, statErr := os.Stat(appendFile); statErr != nil {
			return fmt.Errorf("unable to find file: %s - %s", appendFile, statErr.Error())
		}

		// 检查是否有重复函数名
		if err := duplicateFunctionName(functionName, appendFile); err != nil {
			return err
		}

		fileName = appendFile
		outputMsg = fmt.Sprintf("Stack file updated: %s\n", fileName)

	} else {
		// 新建模式：获取网关地址
		gateway = getGatewayURL(gateway, defaultGateway, gateway, os.Getenv(openFaaSURLEnvironment))
		fileName = yamlFile
		outputMsg = fmt.Sprintf("Stack file written: %s\n", fileName)
	}

	// 4. 设置函数目录，默认为函数名
	if len(handlerDir) == 0 {
		handlerDir = functionName
	}

	// 目录不能已存在
	if _, err := os.Stat(handlerDir); err == nil {
		return fmt.Errorf("folder: %s already exists", handlerDir)
	}

	// YAML 文件不能已存在（非追加模式）
	if _, err := os.Stat(fileName); err == nil && !appendMode {
		return fmt.Errorf("file: %s already exists. Try \"faas-cli new --append %s\" instead", fileName, fileName)
	}

	// 创建函数目录
	if err := os.Mkdir(handlerDir, 0700); err != nil {
		return fmt.Errorf("folder: could not create %s : %s", handlerDir, err)
	}

	fmt.Printf("Folder: %s created.\n", handlerDir)

	// 更新 .gitignore 文件
	if err := updateGitignore(); err != nil {
		return fmt.Errorf("got unexpected error while updating .gitignore file: %s", err)
	}

	// 读取语言模板配置
	pathToTemplateYAML := fmt.Sprintf("./template/%s/template.yml", language)
	if _, err := os.Stat(pathToTemplateYAML); err != nil && os.IsNotExist(err) {
		return err
	}

	langTemplate, err := stack.ParseYAMLForLanguageTemplate(pathToTemplateYAML)
	if err != nil {
		return fmt.Errorf("error reading language template: %s", err.Error())
	}

	// 获取模板中的处理函数文件夹名称，默认为 function
	templateHandlerFolder := "function"
	if len(langTemplate.HandlerFolder) > 0 {
		templateHandlerFolder = langTemplate.HandlerFolder
	}

	fromTemplateHandler := filepath.Join("template", language, templateHandlerFolder)

	// 从模板复制代码到新函数目录
	if err := builder.CopyFiles(fromTemplateHandler, handlerDir); err != nil {
		return fmt.Errorf("error copying template handler: %s", err)
	}

	// 打印欢迎 Logo
	printLogo()
	fmt.Printf("\nFunction created in folder: %s\n", handlerDir)

	// 构建镜像名称
	imageName := fmt.Sprintf("%s:latest", functionName)

	// 获取镜像前缀（命令行 > 环境变量）
	imagePrefixVal := getPrefixValue()

	if imagePrefixVal = strings.TrimSpace(imagePrefixVal); len(imagePrefixVal) > 0 {
		imageName = fmt.Sprintf("%s/%s", imagePrefixVal, imageName)
	}

	// 构造函数配置
	function := stack.Function{
		Name:     functionName,
		Handler:  "./" + handlerDir,
		Language: language,
		Image:    imageName,
	}

	// 设置资源限制
	if len(memoryLimit) > 0 || len(cpuLimit) > 0 {
		function.Limits = &stack.FunctionResources{
			CPU:    cpuLimit,
			Memory: memoryLimit,
		}
	}

	// 设置资源请求
	if len(memoryRequest) > 0 || len(cpuRequest) > 0 {
		function.Requests = &stack.FunctionResources{
			CPU:    cpuRequest,
			Memory: memoryRequest,
		}
	}

	// 生成 YAML 内容
	yamlContent := prepareYAMLContent(appendMode, gateway, &function)

	// 写入 YAML 文件
	f, err := os.OpenFile("./"+fileName, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("could not open file '%s' %s", fileName, err)
	}

	_, stackWriteErr := f.Write([]byte(yamlContent))
	if stackWriteErr != nil {
		return fmt.Errorf("error writing stack file %s", stackWriteErr)
	}

	fmt.Print(outputMsg)

	// 非安静模式下显示模板说明
	if !quiet {
		languageTemplate, _ := stack.LoadLanguageTemplate(language)

		if languageTemplate.WelcomeMessage != "" {
			fmt.Printf("\nNotes:\n")
			fmt.Printf("%s\n", languageTemplate.WelcomeMessage)
		}
	}

	return nil
}

// getPrefixValue 获取镜像前缀：优先命令行 --prefix，其次环境变量 OPENFAAS_PREFIX
func getPrefixValue() string {
	prefix := ""
	if len(imagePrefix) > 0 {
		return imagePrefix
	}

	if val, ok := os.LookupEnv("OPENFAAS_PREFIX"); ok && len(val) > 0 {
		prefix = val
	}
	return prefix
}

// prepareYAMLContent 生成 stack.yaml 内容，支持新建/追加两种模式
func prepareYAMLContent(appendMode bool, gateway string, function *stack.Function) (yamlContent string) {

	yamlContent = `  ` + function.Name + `:
    lang: ` + function.Language + `
    handler: ` + function.Handler + `
    image: ` + function.Image + `
`

	// 追加资源请求配置
	if function.Requests != nil && (len(function.Requests.CPU) > 0 || len(function.Requests.Memory) > 0) {
		yamlContent += "    requests:\n"
		if len(function.Requests.CPU) > 0 {
			yamlContent += `      cpu: ` + function.Requests.CPU + "\n"
		}

		if len(function.Requests.Memory) > 0 {
			yamlContent += `      memory: ` + function.Requests.Memory + "\n"
		}
	}

	// 追加资源限制配置
	if function.Limits != nil && (len(function.Limits.CPU) > 0 || len(function.Limits.Memory) > 0) {
		yamlContent += "    limits:\n"
		if len(function.Limits.CPU) > 0 {
			yamlContent += `      cpu: ` + function.Limits.CPU + "\n"
		}

		if len(function.Limits.Memory) > 0 {
			yamlContent += `      memory: ` + function.Limits.Memory + "\n"
		}
	}

	yamlContent += "\n"

	// 新建模式：添加完整的 stack.yaml 头部
	if !appendMode {
		yamlContent = `version: ` + defaultSchemaVersion + `
provider:
  name: openfaas
  gateway: ` + gateway + `
functions:
` + yamlContent
	}

	return yamlContent
}

// printAvailableTemplates 按字母顺序排序并打印可用模板
func printAvailableTemplates(availableTemplates []string) string {
	var result string
	sort.Slice(availableTemplates, func(i, j int) bool {
		return availableTemplates[i] < availableTemplates[j]
	})
	for _, template := range availableTemplates {
		result += fmt.Sprintf("- %s\n", template)
	}
	return result
}

// duplicateFunctionName 检查 YAML 文件中是否存在重复函数名
func duplicateFunctionName(functionName string, appendFile string) error {
	fileBytes, readErr := os.ReadFile(appendFile)
	if readErr != nil {
		return fmt.Errorf("unable to read %s to append, %s", appendFile, readErr)
	}

	services, parseErr := stack.ParseYAMLData(fileBytes, "", "", envsubst)

	if parseErr != nil {
		return fmt.Errorf("Error parsing %s yml file", appendFile)
	}

	if _, exists := services.Functions[functionName]; exists {
		return fmt.Errorf(`
Function %s already exists in %s file. 
Cannot have duplicate function names in same yaml file`, functionName, appendFile)
	}

	return nil
}
