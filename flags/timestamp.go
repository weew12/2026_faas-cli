// Copyright (c) OpenFaaS Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package flags 为 OpenFaaS CLI 提供自定义的命令行标志类型实现
package flags

import "time"

// TimestampFlag 实现 pflag.Value 接口，用于接收并验证 RFC3339 格式的时间戳命令行参数
type TimestampFlag string

// Type 实现 pflag.Value 接口
// 返回参数类型名称，用于命令行帮助信息展示
func (t *TimestampFlag) Type() string {
	return "timestamp"
}

// String 实现 fmt.Stringer 接口
// 返回时间戳的字符串形式，空指针返回空字符串
func (t *TimestampFlag) String() string {
	if t == nil {
		return ""
	}
	return string(*t)
}

// Set 实现 pflag.Value 接口
// 解析并验证输入值是否为合法的 RFC3339 时间戳，不合法则返回错误
func (t *TimestampFlag) Set(value string) error {
	_, err := time.Parse(time.RFC3339, value)
	if err == nil {
		*t = TimestampFlag(value)
	}
	return err
}

// AsTime 将时间戳转换为 time.Time 类型并返回
// 因为参数已经在 Set 阶段验证过，所以此处忽略解析错误
func (t TimestampFlag) AsTime() time.Time {
	v, _ := time.Parse(time.RFC3339, t.String())
	return v
}
