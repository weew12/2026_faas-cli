// Copyright (c) OpenFaaS Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 日志查看命令
// 本文件实现 logs 命令：实时获取函数日志，支持格式化输出、时间过滤、持续跟踪
package commands

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/openfaas/faas-cli/flags"
	"github.com/openfaas/faas-provider/logs"

	"github.com/openfaas/faas-cli/proxy"
	"github.com/spf13/cobra"
)

// 全局变量
var (
	logFlagValues logFlags   // 日志命令的所有标志参数
	nowFunc       = time.Now // 获取当前时间的函数，方便单元测试替换
)

// logFlags 日志命令的所有命令行参数结构体
type logFlags struct {
	instance        string              // 函数实例ID
	since           time.Duration       // 相对时间，如 10m、1h
	sinceTime       flags.TimestampFlag // 绝对时间（RFC3339）
	tail            bool                // 是否持续跟踪日志
	lines           int                 // 显示最近多少行日志
	token           string              // JWT 认证令牌
	logFormat       flags.LogFormat     // 日志输出格式（text/json/keyvalue）
	includeName     bool                // 是否输出函数名
	includeInstance bool                // 是否输出实例ID
	timeFormat      flags.TimeFormat    // 时间戳格式
}

// init 初始化日志命令的参数并注册到主命令
func init() {
	initLogCmdFlags(functionLogsCmd)
	faasCmd.AddCommand(functionLogsCmd)
}

// functionLogsCmd 函数日志查看主命令
var functionLogsCmd = &cobra.Command{
	Use:   `logs <NAME> [--tls-no-verify] [--gateway] [--output=text/json]`,
	Short: "Fetch logs for functions",
	Long:  "Fetch logs for a given function name in plain text or JSON format.",
	Example: `  faas-cli logs FN
  faas-cli logs FN --output=json
  faas-cli logs FN --lines=5
  faas-cli logs FN --tail=false --since=10m
  faas-cli logs FN --tail=false --since=2010-01-01T00:00:00Z
`,
	Args:    cobra.MaximumNArgs(1), // 最多接收1个参数：函数名
	RunE:    runLogs,               // 执行逻辑
	PreRunE: noopPreRunCmd,         // 前置校验
}

// noopPreRunCmd 前置校验：必须提供函数名
func noopPreRunCmd(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("function name is required")
	}
	return nil
}

// initLogCmdFlags 初始化日志命令的所有标志
// 独立成函数方便单元测试重置参数
func initLogCmdFlags(cmd *cobra.Command) {
	logFlagValues = logFlags{}

	// 基础参数
	cmd.Flags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL starting with http(s)://")
	cmd.Flags().StringVarP(&functionNamespace, "namespace", "n", "", "Namespace of the function")

	// TLS 验证
	cmd.Flags().BoolVar(&tlsInsecure, "tls-no-verify", false, "Disable TLS validation")

	// 时间范围
	cmd.Flags().DurationVar(&logFlagValues.since, "since", 0*time.Second, "return logs newer than a relative duration like 5s")
	cmd.Flags().Var(&logFlagValues.sinceTime, "since-time", "include logs since the given timestamp (RFC3339)")

	// 日志行数
	cmd.Flags().IntVar(&logFlagValues.lines, "lines", -1, "number of recent log lines file to display. Defaults to -1, unlimited if <=0")

	// 持续跟踪
	cmd.Flags().BoolVarP(&logFlagValues.tail, "tail", "t", true, "tail logs and continue printing new logs until the end of the request, up to 30s")

	// 认证
	cmd.Flags().StringVarP(&logFlagValues.token, "token", "k", "", "Pass a JWT token to use instead of basic auth")

	// 输出格式
	logFlagValues.timeFormat = flags.TimeFormat(time.RFC3339)
	cmd.Flags().VarP(&logFlagValues.logFormat, "output", "o", "output logs as (plain|keyvalue|json), JSON includes all available keys")
	cmd.Flags().Var(&logFlagValues.timeFormat, "time-format", "string format for the timestamp, any go time format is allowed")
	cmd.Flags().BoolVar(&logFlagValues.includeName, "name", false, "print the function name")
	cmd.Flags().BoolVar(&logFlagValues.includeInstance, "instance", false, "print the function instance/id")
}

// runLogs 执行获取日志的核心逻辑
func runLogs(cmd *cobra.Command, args []string) error {
	// 获取网关地址
	gatewayAddress := getGatewayURL(gateway, defaultGateway, "", os.Getenv(openFaaSURLEnvironment))
	// 输出 TLS 不安全提示
	if msg := checkTLSInsecure(gatewayAddress, tlsInsecure); len(msg) > 0 {
		fmt.Println(msg)
	}

	// 从命令行参数构造日志请求
	logRequest := logRequestFromFlags(cmd, args)

	// 创建认证客户端
	cliAuth, err := proxy.NewCLIAuth(logFlagValues.token, gatewayAddress)
	if err != nil {
		return err
	}

	// 获取流式日志传输器（处理 TLS 验证）
	transport := getLogStreamingTransport(tlsInsecure)

	// 创建 OpenFaaS 客户端
	cliClient, err := proxy.NewClient(cliAuth, gatewayAddress, transport, nil)
	if err != nil {
		return err
	}

	// 请求日志流
	logEvents, err := cliClient.GetLogs(context.Background(), logRequest)
	if err != nil {
		return err
	}

	// 获取日志格式化器
	formatter := GetLogFormatter(string(logFlagValues.logFormat))

	// 循环输出日志
	for logMsg := range logEvents {
		fmt.Fprintln(os.Stdout, formatter(logMsg, logFlagValues.timeFormat.String(), logFlagValues.includeName, logFlagValues.includeInstance))
	}

	return nil
}

// logRequestFromFlags 从命令行标志构造日志请求结构体
func logRequestFromFlags(cmd *cobra.Command, args []string) logs.Request {
	// 获取命名空间
	ns, err := cmd.Flags().GetString("namespace")
	if err != nil {
		log.Printf("error getting namespace flag %s\n", err.Error())
	}

	// 构造请求
	return logs.Request{
		Name:      args[0],                                                           // 函数名
		Namespace: ns,                                                                // 命名空间
		Tail:      logFlagValues.lines,                                               // 最近行数
		Since:     sinceValue(logFlagValues.sinceTime.AsTime(), logFlagValues.since), // 起始时间
		Follow:    logFlagValues.tail,                                                // 是否持续跟踪
	}
}

// sinceValue 计算日志起始时间：优先绝对时间，其次相对时间
func sinceValue(t time.Time, d time.Duration) *time.Time {
	if !t.IsZero() {
		return &t
	}

	if d.String() != "0s" {
		ts := nowFunc().Add(-1 * d)
		return &ts
	}
	return nil
}

// getLogStreamingTransport 创建日志流式传输的 HTTP 传输器
// 支持关闭 TLS 验证（tls-no-verify）
func getLogStreamingTransport(tlsInsecure bool) http.RoundTripper {
	if tlsInsecure {
		tr := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		}
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: tlsInsecure}
		return tr
	}
	return nil
}
