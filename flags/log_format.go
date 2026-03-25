// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package flags 提供 OpenFaaS CLI 自定义命令行标志类型
// 包含日志格式等可复用的 pflag 兼容类型定义
package flags

import (
	"fmt"
	"strings"
)

// LogFormat 定义日志输出格式类型，实现 pflag.Value 接口
// 用于命令行参数解析，支持 plain/keyvalue/json 三种格式
type LogFormat string

// 支持的日志格式常量
const (
	PlainLogFormat    LogFormat = "plain"    // 纯文本日志格式
	KeyValueLogFormat LogFormat = "keyvalue" // 键值对日志格式
	JSONLogFormat     LogFormat = "json"     // JSON 结构化日志格式
)

// Type 实现 pflag.Value 接口
// 返回标志类型名称，用于帮助信息展示
func (l *LogFormat) Type() string {
	return "logformat"
}

// String 实现 fmt.Stringer 接口
// 返回 LogFormat 的字符串表示，空指针返回空字符串
func (l *LogFormat) String() string {
	if l == nil {
		return ""
	}
	return string(*l)
}

// Set 实现 pflag.Value 接口
// 解析并设置日志格式，仅支持 plain/keyvalue/json 有效值
// 输入值会自动转为小写，非法值返回错误
func (l *LogFormat) Set(value string) error {
	switch strings.ToLower(value) {
	case "plain", "keyvalue", "json":
		*l = LogFormat(value)
	default:
		return fmt.Errorf("unknown log format: '%s'", value)
	}
	return nil
}
