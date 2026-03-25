// Copyright (c) OpenFaaS Author(s) 2018. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package v1 定义 OpenFaaS v1 版本的 CRD 数据结构
package v1

import (
	"github.com/openfaas/faas-cli/schema"
	"github.com/openfaas/go-sdk/stack"
)

// APIVersionLatest CRD 最新 API 版本常量
const APIVersionLatest = "openfaas.com/v1"

// Spec 定义函数资源的核心规格
type Spec struct {
	// Name 函数名称
	Name string `yaml:"name"`
	// Image 函数容器镜像地址
	Image string `yaml:"image"`

	// Environment 函数环境变量
	Environment map[string]string `yaml:"environment,omitempty"`

	// Labels 函数标签
	Labels *map[string]string `yaml:"labels,omitempty"`

	// Annotations 函数注解
	Annotations *map[string]string `yaml:"annotations,omitempty"`

	// Limits 函数资源限制（CPU/内存）
	Limits *stack.FunctionResources `yaml:"limits,omitempty"`

	// Requests 函数资源请求值
	Requests *stack.FunctionResources `yaml:"requests,omitempty"`

	// Constraints 调度约束条件
	Constraints *[]string `yaml:"constraints,omitempty"`

	// Secrets 函数可用的密钥列表
	Secrets []string `yaml:"secrets,omitempty"`

	// ReadOnlyRootFilesystem 是否将根文件系统设为只读
	ReadOnlyRootFilesystem bool `yaml:"readOnlyRootFilesystem,omitempty"`
}

// CRD OpenFaaS 自定义资源顶层定义
type CRD struct {
	// APIVersion API 版本
	APIVersion string `yaml:"apiVersion"`
	// Kind 资源类型
	Kind     string          `yaml:"kind"`
	Metadata schema.Metadata `yaml:"metadata"`
	Spec     Spec            `yaml:"spec"`
}
