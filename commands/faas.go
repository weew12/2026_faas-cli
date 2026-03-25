// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"syscall"

	"github.com/moby/term"
	"github.com/openfaas/faas-cli/version"
	"github.com/openfaas/go-sdk/stack"
	"github.com/spf13/cobra"
)

// 常量定义：默认网关地址、配置文件名、schema版本
const (
	defaultGateway       = "http://127.0.0.1:8080" // 默认 OpenFaaS 网关地址
	defaultNetwork       = ""                      // 默认网络
	defaultYML           = "stack.yml"             // 默认配置文件（后缀 yml）
	defaultYAML          = "stack.yaml"            // 默认配置文件（后缀 yaml）
	defaultSchemaVersion = "1.0"                   // 默认 YAML  schema 版本
)

// 全局标志：所有命令都可以使用
var (
	yamlFile string // YAML 配置文件路径
	regex    string // 正则匹配函数名
	filter   string // 通配符匹配函数名
)

// 全局标志：部分命令使用
var (
	fprocess     string // 函数进程名
	functionName string // 函数名
	handlerDir   string // 处理器目录
	network      string // 网络
	gateway      string // 网关地址
	handler      string // 函数处理器
	image        string // 容器镜像
	imagePrefix  string // 镜像前缀
	language     string // 函数语言
	tlsInsecure  bool   // 禁用 TLS 证书验证
)

// services 从 stack.yaml 解析出来的服务配置
var services *stack.Services

// stat 文件状态检查函数（可被测试覆盖）
var stat = func(filename string) (os.FileInfo, error) {
	return os.Stat(filename)
}

// resetForTest 测试重置全局变量（内部使用）
func resetForTest() {
	yamlFile = ""
	regex = ""
	filter = ""
	version.Version = ""
	shortVersion = false
	appendFile = ""
}

func init() {
	// 初始化终端标准流
	term.StdStreams()

	// 为根命令添加持久标志：所有子命令都能使用
	faasCmd.PersistentFlags().StringVarP(&yamlFile, "yaml", "f", "", "Path to YAML file describing function(s)")
	faasCmd.PersistentFlags().StringVarP(&regex, "regex", "", "", "Regex to match with function names in YAML file")
	faasCmd.PersistentFlags().StringVarP(&filter, "filter", "", "", "Wildcard to match with function names in YAML file")

	// 设置 Bash 自动补全：只显示 yaml/yml 文件
	validYAMLFilenames := []string{"yaml", "yml"}
	_ = faasCmd.PersistentFlags().SetAnnotation("yaml", cobra.BashCompFilenameExt, validYAMLFilenames)
}

// Execute 执行 CLI 入口函数
func Execute(customArgs []string) {
	// 检查并自动设置默认的 stack.yaml / stack.yml
	checkAndSetDefaultYaml()

	// 关闭默认的错误和使用说明打印
	faasCmd.SilenceUsage = true
	faasCmd.SilenceErrors = true
	faasCmd.SetArgs(customArgs[1:])

	args1 := os.Args[1:]
	// 查找用户输入的命令
	cmd1, _, _ := faasCmd.Find(args1)

	// 查找插件
	plugins, err := getPlugins()
	if err != nil {
		log.Fatal(err)
	}

	// 如果命令不是内置命令，则尝试运行插件
	if cmd1 != nil && len(args1) > 0 {
		found := ""
		for _, plugin := range plugins {
			pluginName := args1[0]
			// Windows 插件需要加 .exe 后缀
			if runtime.GOOS == "windows" {
				pluginName = fmt.Sprintf("%s.exe", args1[0])
			}

			if path.Base(plugin) == pluginName {
				found = plugin
			}
		}

		// 如果找到插件，则执行插件
		if len(found) > 0 {
			// Windows 不支持 syscall.Exec，所以用 Command 运行
			if runtime.GOOS == "windows" {
				cmd := exec.Command(found, os.Args[2:]...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					var exitErr *exec.ExitError
					if errors.As(err, &exitErr) {
						os.Exit(exitErr.ExitCode())
					} else {
						fmt.Println("Error from plugin", err)
						os.Exit(127)
					}
				}
				return
			} else {
				// Linux/macOS 使用 syscall.Exec 替换进程执行插件
				if err := syscall.Exec(found, append([]string{found}, os.Args[2:]...), os.Environ()); err != nil {
					fmt.Fprintf(os.Stderr, "Error from plugin: %v", err)
					os.Exit(127)
				}
				return
			}
		}
	}

	// 执行内置命令
	if err := faasCmd.Execute(); err != nil {
		e := err.Error()
		fmt.Println(strings.ToUpper(e[:1]) + e[1:])
		os.Exit(1)
	}
}

// checkAndSetDefaultYaml 如果当前目录存在 stack.yaml/yml，自动设为默认配置文件
func checkAndSetDefaultYaml() {
	if _, err := stat(defaultYAML); err == nil {
		yamlFile = defaultYAML
	} else if _, err := stat(defaultYML); err == nil {
		yamlFile = defaultYML
	}
}

// faasCmd 根命令：faas-cli
var faasCmd = &cobra.Command{
	Use:   "faas-cli",
	Short: "Manage your OpenFaaS functions from the command line",
	Long: `
Manage your OpenFaaS functions from the command line`,
	Run: runFaas,
}

// runFaas 根命令默认执行：打印 logo + 帮助信息
func runFaas(cmd *cobra.Command, args []string) {
	printLogo()
	cmd.Help()
}

// getPlugins 获取 ~/.openfaas/plugins 目录下的所有插件
func getPlugins() ([]string, error) {
	plugins := []string{}
	var pluginHome string

	// 不同系统插件目录
	if runtime.GOOS == "windows" {
		pluginHome = os.Expand("$HOMEPATH/.openfaas/plugins", os.Getenv)
	} else {
		pluginHome = os.ExpandEnv("$HOME/.openfaas/plugins")
	}

	// 目录不存在则返回空列表
	if _, err := os.Stat(pluginHome); err != nil && os.IsNotExist(err) {
		return plugins, nil
	}

	// 读取目录文件
	res, err := os.ReadDir(pluginHome)
	if err != nil {
		return nil, err
	}

	// 拼接插件路径
	for _, file := range res {
		plugins = append(plugins, path.Join(pluginHome, file.Name()))
	}

	return plugins, nil
}
