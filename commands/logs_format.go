// Copyright (c) OpenFaaS Author(s) 2025. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"encoding/json"
	"strings"

	"github.com/openfaas/faas-cli/flags"
	"github.com/openfaas/faas-provider/logs"
)

// LogFormatter 日志格式化器函数类型定义
// 接收日志消息、时间格式、是否显示函数名、是否显示实例ID，返回格式化后的字符串
type LogFormatter func(msg logs.Message, timeFormat string, includeName, includeInstance bool) string

// GetLogFormatter 根据格式名称获取对应的日志格式化器
// 支持：json / keyvalue / 默认(plain)
func GetLogFormatter(name string) LogFormatter {
	switch name {
	case string(flags.JSONLogFormat):
		return JSONFormatMessage
	case string(flags.KeyValueLogFormat):
		return KeyValueFormatMessage
	default:
		return PlainFormatMessage
	}
}

// JSONFormatMessage JSON 格式日志输出
// 忽略所有显示选项，直接将完整日志序列化为 JSON 字符串
func JSONFormatMessage(msg logs.Message, timeFormat string, includeName, includeInstance bool) string {
	// 结构体简单，不会出现序列化错误，忽略错误
	b, _ := json.Marshal(msg)
	return string(b)
}

// KeyValueFormatMessage 键值对格式日志
// 输出格式：timestamp="..." name="..." instance="..." text="..."
func KeyValueFormatMessage(msg logs.Message, timeFormat string, includeName, includeInstance bool) string {
	var b strings.Builder

	// 写入时间戳
	if timeFormat != "" {
		b.WriteString("timestamp=\"")
		b.WriteString(msg.Timestamp.Format(timeFormat))
		b.WriteString("\" ")
	}

	// 写入函数名
	if includeName {
		b.WriteString("name=\"")
		b.WriteString(msg.Name)
		b.WriteString("\" ")
	}

	// 写入实例ID
	if includeInstance {
		b.WriteString("instance=\"")
		b.WriteString(msg.Instance)
		b.WriteString("\" ")
	}

	// 写入日志内容
	b.WriteString("text=\"")
	b.WriteString(strings.TrimRight(msg.Text, "\n"))
	b.WriteString("\" ")

	return b.String()
}

// PlainFormatMessage 普通简洁格式日志
// 输出格式：<时间戳> <函数名> (<实例ID>) <日志内容>
func PlainFormatMessage(msg logs.Message, timeFormat string, includeName, includeInstance bool) string {
	var b strings.Builder

	// 写入时间戳
	if timeFormat != "" {
		b.WriteString(msg.Timestamp.Format(timeFormat))
		b.WriteString(" ")
	}

	// 写入函数名
	if includeName {
		b.WriteString(msg.Name)
		b.WriteString(" ")
	}

	// 写入实例ID
	if includeInstance {
		b.WriteString("(")
		b.WriteString(msg.Instance)
		b.WriteString(")")
		b.WriteString(" ")
	}

	// 写入日志主体内容
	b.WriteString(msg.Text)

	// 去除末尾换行符
	return strings.TrimRight(b.String(), "\n")
}
