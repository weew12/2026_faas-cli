// Copyright (c) OpenFaaS Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package util 提供通用工具函数
// 包含键值对解析、map合并、切片去重合并等通用能力
package util

import (
	"fmt"
	"strings"
)

// ParseMap 将 key=value 格式的字符串数组解析为 map[string]string
// 参数：
//
//	envvars - 待解析的 key=value 字符串切片
//	keyName - 错误提示中使用的项名称（如 label/env）
//
// 返回：解析后的map，或格式错误时返回错误
func ParseMap(envvars []string, keyName string) (map[string]string, error) {
	result := make(map[string]string)
	for _, envvar := range envvars {
		// 按第一个 = 分割键值，保留值中的 = 符号
		s := strings.SplitN(strings.TrimSpace(envvar), "=", 2)
		if len(s) != 2 {
			return nil, fmt.Errorf("label format is not correct, needs key=value")
		}
		envvarName := s[0]
		envvarValue := s[1]

		// 校验键不能为空
		if len(envvarName) == 0 {
			return nil, fmt.Errorf("empty %s name: [%s]", keyName, envvar)
		}
		// 校验值不能为空
		if len(envvarValue) == 0 {
			return nil, fmt.Errorf("empty %s value: [%s]", keyName, envvar)
		}

		result[envvarName] = envvarValue
	}
	return result, nil
}

// MergeMap 合并两个 string 类型 map，overlay 中的键会覆盖 base 中的同名键
// 返回全新分配的合并后map，不会修改原始map
func MergeMap(base map[string]string, overlay map[string]string) map[string]string {
	merged := make(map[string]string)

	// 先加入基础map
	for k, v := range base {
		merged[k] = v
	}
	// 覆盖/追加覆盖map
	for k, v := range overlay {
		merged[k] = v
	}

	return merged
}

// MergeSlice 合并两个字符串切片，自动去重
// overlay 切片优先级更高，且元素唯一，values 中未重复的元素追加到末尾
func MergeSlice(values []string, overlay []string) []string {
	results := []string{}
	// 记录已添加的元素，用于去重
	added := make(map[string]bool)

	// 优先加入 overlay 所有元素
	for _, value := range overlay {
		results = append(results, value)
		added[value] = true
	}

	// 加入 values 中未存在的元素
	for _, value := range values {
		if !added[value] {
			results = append(results, value)
			added[value] = true
		}
	}

	return results
}
