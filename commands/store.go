// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件提供函数商店（store）公共工具方法，用于获取、过滤、查询商店函数
package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/openfaas/faas-cli/proxy"
	storeV2 "github.com/openfaas/faas-cli/schema/store/v2"
	"github.com/spf13/cobra"
)

// 全局命令行参数与配置
var (
	storeAddress     string      // 函数商店 API 地址
	verbose          bool        // 详细输出模式
	storeDeployFlags DeployFlags // 部署商店函数时的参数
	Platform         string      // 编译时注入的目标平台

	// shortPlatform 长平台名与商店短平台名映射表
	shortPlatform = map[string]string{
		"linux/arm/v6": "armhf",
		"linux/amd64":  "x86_64",
		"linux/arm64":  "arm64",
	}
)

const (
	defaultStore      = "https://raw.githubusercontent.com/openfaas/store/master/functions.json" // 默认官方商店地址
	maxDescriptionLen = 40                                                                       // 函数描述最大展示长度
)

var platformValue string // 命令行传入的平台参数

// init 初始化 store 根命令，注册全局标志并添加到根命令
func init() {
	storeCmd.PersistentFlags().StringVarP(&storeAddress, "url", "u", defaultStore, "Alternative Store URL starting with http(s)://")
	storeCmd.PersistentFlags().StringVarP(&platformValue, "platform", "p", Platform, "Target platform for store")

	faasCmd.AddCommand(storeCmd)
}

// storeCmd OpenFaaS 函数商店根命令，用于浏览、部署商店中的函数
var storeCmd = &cobra.Command{
	Use:   `store`,
	Short: "OpenFaaS store commands",
	Long:  "Allows browsing and deploying OpenFaaS functions from a store",
}

// storeList 从商店 URL 获取函数列表
// 发起 HTTP 请求并解析商店 V2 版本 JSON 数据
func storeList(store string) ([]storeV2.StoreFunction, error) {

	var storeData storeV2.Store

	store = strings.TrimRight(store, "/")

	timeout := 60 * time.Second
	tlsInsecure := false

	client := proxy.MakeHTTPClient(&timeout, tlsInsecure)

	res, err := client.Get(store)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to OpenFaaS store at URL: %s", store)
	}

	if res.Body != nil {
		defer func() {
			_, _ = io.Copy(io.Discard, res.Body) // 排空响应体
			_ = res.Body.Close()
		}()
	}

	switch res.StatusCode {
	case http.StatusOK:
		bytesOut, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("cannot read result from OpenFaaS store at URL: %s", store)
		}

		jsonErr := json.Unmarshal(bytesOut, &storeData)
		if jsonErr != nil {
			return nil, fmt.Errorf("cannot parse result from OpenFaaS store at URL: %s\n%s", store, jsonErr.Error())
		}
	default:
		bytesOut, err := io.ReadAll(res.Body)
		if err == nil {
			return nil, fmt.Errorf("server returned unexpected status code: %d - %s", res.StatusCode, string(bytesOut))
		}
	}

	return storeData.Functions, nil
}

// filterStoreList 根据平台过滤商店函数列表
// 只返回支持指定平台的函数
func filterStoreList(functions []storeV2.StoreFunction, platform string) []storeV2.StoreFunction {
	var filteredList []storeV2.StoreFunction

	for _, function := range functions {

		_, ok := getValueIgnoreCase(function.Images, platform)

		if ok {
			filteredList = append(filteredList, function)
		}
	}

	return filteredList
}

// getValueIgnoreCase 忽略大小写从 map 中获取值
// 用于兼容平台名称大小写差异
func getValueIgnoreCase(kv map[string]string, key string) (string, bool) {
	for k, v := range kv {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return "", false
}

// storeFindFunction 在函数列表中按名称/标题查找函数
func storeFindFunction(functionName string, storeItems []storeV2.StoreFunction) *storeV2.StoreFunction {
	var item storeV2.StoreFunction

	for _, item = range storeItems {
		if item.Name == functionName || item.Title == functionName {
			return &item
		}
	}

	return nil
}

// getPlatform 获取当前 CLI 构建时的平台
// 未指定则返回默认平台 x86_64
func getPlatform() string {
	if len(Platform) == 0 {
		return mainPlatform
	}
	return Platform
}

// getTargetPlatform 获取最终使用的目标平台
// 自动将长平台名映射为商店支持的短名称
func getTargetPlatform(inputPlatform string) string {
	if len(inputPlatform) == 0 {
		currentPlatform := getPlatform()
		target, ok := shortPlatform[currentPlatform]
		if ok {
			return target
		}
		return currentPlatform
	}
	return inputPlatform
}

// getStorePlatforms 获取商店中所有函数支持的平台列表（去重）
func getStorePlatforms(functions []storeV2.StoreFunction) []string {
	var distinctPlatformMap = make(map[string]bool)
	var result []string

	for _, function := range functions {
		for key := range function.Images {
			_, exists := distinctPlatformMap[key]

			if !exists {
				distinctPlatformMap[key] = true
				result = append(result, key)
			}
		}
	}

	return result
}
