// Copyright (c) OpenFaaS Author(s) 2018. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/openfaas/faas-cli/proxy"
	"github.com/openfaas/faas-cli/schema"
	"github.com/openfaas/faas-provider/types"
	"github.com/openfaas/go-sdk/stack"

	"github.com/spf13/cobra"
)

// init 初始化命令参数
func init() {
	describeCmd.Flags().StringVar(&functionName, "name", "", "Name of the function")
	describeCmd.Flags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL starting with http(s)://")
	describeCmd.Flags().BoolVar(&tlsInsecure, "tls-no-verify", false, "Disable TLS validation")
	describeCmd.Flags().BoolVar(&envsubst, "envsubst", true, "Substitute environment variables in stack.yaml file")
	describeCmd.Flags().StringVarP(&token, "token", "k", "", "Pass a JWT token to use instead of basic auth")
	describeCmd.Flags().StringVarP(&functionNamespace, "namespace", "n", "", "Namespace of the function")
	describeCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	// 将 describe 命令注册到根命令
	faasCmd.AddCommand(describeCmd)
}

// describeCmd 展示函数详细信息的子命令
var describeCmd = &cobra.Command{
	Use:   "describe FUNCTION_NAME [--gateway GATEWAY_URL]",
	Short: "Describe an OpenFaaS function",
	Long:  `Display details of an OpenFaaS function`,
	Example: `faas-cli describe figlet
faas-cli describe env --gateway http://127.0.0.1:8080
faas-cli describe echo -g http://127.0.0.1.8080`,
	PreRunE: preRunDescribe,
	RunE:    runDescribe,
}

// preRunDescribe 执行前的钩子，无额外逻辑
func preRunDescribe(cmd *cobra.Command, args []string) error {
	return nil
}

// runDescribe 执行函数描述的核心逻辑
func runDescribe(cmd *cobra.Command, args []string) error {
	// 必须传入函数名
	if len(args) < 1 {
		return fmt.Errorf("please provide a name for the function")
	}
	var yamlGateway string
	var services stack.Services
	functionName = args[0]

	// 如果指定了 yaml 文件，解析并获取其中的网关地址
	if len(yamlFile) > 0 {
		parsedServices, err := stack.ParseYAMLFile(yamlFile, regex, filter, envsubst)
		if err != nil {
			return err
		}

		if parsedServices != nil {
			services = *parsedServices
			yamlGateway = services.Provider.GatewayURL
		}
	}
	// 确定最终使用的网关地址
	gatewayAddress := getGatewayURL(gateway, defaultGateway, yamlGateway, os.Getenv(openFaaSURLEnvironment))
	// 创建认证信息
	cliAuth, err := proxy.NewCLIAuth(token, gatewayAddress)
	if err != nil {
		return err
	}
	// 获取 HTTP 传输配置
	transport := GetDefaultCLITransport(tlsInsecure, &commandTimeout)
	// 创建 API 客户端
	cliClient, err := proxy.NewClient(cliAuth, gatewayAddress, transport, &commandTimeout)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// 查询函数的详细信息
	function, err := cliClient.GetFunctionInfo(ctx, functionName, functionNamespace)
	if err != nil {
		return err
	}

	// 从函数列表接口获取准确的调用次数
	functionList, err := cliClient.ListFunctions(ctx, functionNamespace)
	if err != nil {
		return err
	}

	var invocationCount int
	for _, fn := range functionList {
		if fn.Name == function.Name {
			invocationCount = int(fn.InvocationCount)
			break
		}
	}

	// 判断函数是否就绪
	var status = "Not Ready"
	if function.AvailableReplicas > 0 {
		status = "Ready"
	}

	// 构造函数的同步和异步访问地址
	url, asyncURL := getFunctionURLs(gatewayAddress, functionName, functionNamespace)

	// 组装完整的函数描述对象
	funcDesc := schema.FunctionDescription{
		FunctionStatus:  function,
		Status:          status,
		InvocationCount: int(invocationCount),
		URL:             url,
		AsyncURL:        asyncURL,
	}

	// 格式化输出所有信息
	printFunctionDescription(cmd.OutOrStdout(), funcDesc, verbose)

	return nil
}

// getFunctionURLs 构造函数的同步和异步调用 URL
func getFunctionURLs(gateway string, functionName string, functionNamespace string) (string, string) {
	// 去掉网关地址末尾的斜杠
	gateway = strings.TrimRight(gateway, "/")

	url := gateway + "/function/" + functionName
	asyncURL := gateway + "/async-function/" + functionName

	// 如果有命名空间，追加到 URL 后
	if functionNamespace != "" {
		url += "." + functionNamespace
		asyncURL += "." + functionNamespace
	}

	return url, asyncURL
}

// printFunctionDescription 使用表格格式美观打印函数信息
func printFunctionDescription(dst io.Writer, funcDesc schema.FunctionDescription, verbose bool) {
	// 使用 tabwriter 实现对齐输出
	w := tabwriter.NewWriter(dst, 0, 0, 1, ' ', tabwriter.TabIndent)
	defer w.Flush()

	out := printer{
		w:       w,
		verbose: verbose,
	}

	// 处理函数进程名称
	process := "<default>"
	if funcDesc.EnvProcess != "" {
		process = funcDesc.EnvProcess
	}

	// 输出基础信息
	out.Printf("Name:\t%s\n", funcDesc.Name)
	out.Printf("Status:\t%s\n", funcDesc.Status)
	out.Printf("Replicas:\t%s\n", strconv.Itoa(int(funcDesc.Replicas)))
	out.Printf("Available Replicas:\t%s\n", strconv.Itoa(int(funcDesc.AvailableReplicas)))
	out.Printf("Invocations:\t%s\n", strconv.Itoa(int(funcDesc.InvocationCount)))
	out.Printf("Image:\t%s\n", funcDesc.Image)
	out.Printf("Function Process:\t%s\n", process)
	out.Printf("URL:\t%s\n", funcDesc.URL)
	out.Printf("Async URL:\t%s\n", funcDesc.AsyncURL)

	// 输出标签、注解等扩展信息
	if funcDesc.Labels != nil {
		out.Printf("Labels", *funcDesc.Labels)
	} else {
		out.Printf("Labels", map[string]string{})
	}
	if funcDesc.Annotations != nil {
		out.Printf("Annotations", *funcDesc.Annotations)
	} else {
		out.Printf("Annotations", map[string]string{})
	}
	out.Printf("Constraints", funcDesc.Constraints)
	out.Printf("Environment", funcDesc.EnvVars)
	out.Printf("Secrets", funcDesc.Secrets)
	out.Printf("Requests", funcDesc.Requests)
	out.Printf("Limits", funcDesc.Limits)
	out.Printf("", funcDesc.Usage)
}

// printer 带详细模式的输出工具结构
type printer struct {
	verbose bool
	w       io.Writer
}

// Printf 根据不同类型自动选择输出格式
func (p *printer) Printf(format string, a interface{}) {
	switch v := a.(type) {
	case map[string]string:
		printMap(p.w, format, v, p.verbose)
	case []string:
		printList(p.w, format, v, p.verbose)
	case *types.FunctionResources:
		printResources(p.w, format, v, p.verbose)
	case *types.FunctionUsage:
		printUsage(p.w, v, p.verbose)
	default:
		// 非详细模式下不显示空值
		if !p.verbose && isEmpty(a) {
			return
		}

		// 详细模式下空值显示 <none>
		if p.verbose && isEmpty(a) {
			a = "<none>"
		}

		fmt.Fprintf(p.w, format, a)
	}

}

// printUsage 打印函数资源使用情况（CPU/内存）
func printUsage(w io.Writer, usage *types.FunctionUsage, verbose bool) {
	if !verbose && usage == nil {
		return
	}

	if usage == nil {
		fmt.Fprintln(w, "Usage:\t <none>")
		return
	}

	fmt.Fprintln(w, "Usage:")
	fmt.Fprintf(w, "  RAM:\t %.2f MB\n", (usage.TotalMemoryBytes / 1024 / 1024))
	cpu := usage.CPU
	if cpu < 0 {
		cpu = 1
	}
	fmt.Fprintf(w, "  CPU:\t %.0f Mi\n", (cpu))
}

// printMap 打印 map 类型数据，如标签、环境变量
func printMap(w io.Writer, name string, m map[string]string, verbose bool) {
	if !verbose && len(m) == 0 {
		return
	}

	if len(m) == 0 {
		fmt.Fprintf(w, "%s:\t <none>\n", name)
		return
	}

	fmt.Fprintf(w, "%s:\n", name)

	// 环境变量按键名排序输出
	if name == "Environment" {
		orderedKeys := generateMapOrder(m)
		for _, keyName := range orderedKeys {
			fmt.Fprintln(w, "\t "+keyName+": "+m[keyName])
		}
		return
	}

	for key, value := range m {
		fmt.Fprintln(w, "\t "+key+": "+value)
	}

	return
}

// printList 打印数组类型数据，如密钥、约束
func printList(w io.Writer, name string, data []string, verbose bool) {
	if !verbose && len(data) == 0 {
		return
	}

	if len(data) == 0 {
		fmt.Fprintf(w, "%s:\t <none>\n", name)
		return
	}

	fmt.Fprintf(w, "%s:\n", name)
	for _, value := range data {
		fmt.Fprintln(w, "\t - "+value)
	}

	return
}

// printResources 打印资源限制/请求配置
func printResources(w io.Writer, name string, data *types.FunctionResources, verbose bool) {
	if !verbose && data == nil {
		return
	}

	header := name + ":"
	if data == nil {
		fmt.Fprintln(w, header+"\t <none>")
		return
	}

	fmt.Fprintln(w, header)
	fmt.Fprintln(w, "\t CPU: "+data.CPU)
	fmt.Fprintln(w, "\t Memory: "+data.Memory)

	return
}

// isEmpty 判断一个值是否为空
func isEmpty(a interface{}) bool {
	v := reflect.ValueOf(a)
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

// generateMapOrder 对 map 的 key 进行排序，保证输出有序
func generateMapOrder(m map[string]string) []string {

	var keyNames []string

	for keyName := range m {
		keyNames = append(keyNames, keyName)
	}

	sort.Strings(keyNames)

	return keyNames
}
