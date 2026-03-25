// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package flags 提供 OpenFaaS CLI 自定义命令行标志类型
// 包含时间格式、日志格式等 pflag 兼容的自定义参数解析类型
package flags

import (
	"strings"
	"time"
)

// TimeFormat 自定义时间格式类型，实现 pflag.Value 接口
// 支持使用标准 RFC 格式别名（如 ansic/rfc3339），也支持传入自定义时间格式字符串
//
// 支持的标准格式别名：
//
//	ANSIC       = "Mon Jan _2 15:04:05 2006"
//	UnixDate    = "Mon Jan _2 15:04:05 MST 2006"
//	RubyDate    = "Mon Jan 02 15:04:05 -0700 2006"
//	RFC822      = "02 Jan 06 15:04 MST"
//	RFC822Z     = "02 Jan 06 15:04 -0700"
//	RFC850      = "Monday, 02-Jan-06 15:04:05 MST"
//	RFC1123     = "Mon, 02 Jan 2006 15:04:05 MST"
//	RFC1123Z    = "Mon, 02 Jan 2006 15:04:05 -0700"
//	RFC3339     = "2006-01-02T15:04:05Z07:00"
//	RFC3339Nano = "2006-01-02T15:04:05.999999999Z07:00"
type TimeFormat string

// Type 实现 pflag.Value 接口
// 返回标志类型名称，用于命令行帮助信息展示
func (l *TimeFormat) Type() string {
	return "timeformat"
}

// String 实现 fmt.Stringer 接口
// 返回 TimeFormat 的字符串值，空指针返回空字符串
func (l *TimeFormat) String() string {
	if l == nil {
		return ""
	}
	return string(*l)
}

// Set 实现 pflag.Value 接口
// 解析输入的时间格式：
// 1. 输入标准别名（不区分大小写）会自动映射为 Go 内置时间格式
// 2. 输入非标准格式字符串，直接作为自定义格式使用
func (l *TimeFormat) Set(value string) error {
	switch strings.ToLower(value) {
	case "ansic":
		*l = TimeFormat(time.ANSIC)
	case "unixdate":
		*l = TimeFormat(time.UnixDate)
	case "rubydate":
		*l = TimeFormat(time.RubyDate)
	case "rfc822":
		*l = TimeFormat(time.RFC822)
	case "rfc822z":
		*l = TimeFormat(time.RFC822Z)
	case "rfc850":
		*l = TimeFormat(time.RFC850)
	case "rfc1123":
		*l = TimeFormat(time.RFC1123)
	case "rfc1123z":
		*l = TimeFormat(time.RFC1123Z)
	case "rfc3339":
		*l = TimeFormat(time.RFC3339)
	case "rfc3339nano":
		*l = TimeFormat(time.RFC3339Nano)
	default:
		*l = TimeFormat(value)
	}
	return nil
}
