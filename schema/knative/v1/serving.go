// Copyright (c) OpenFaaS Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package v1 定义 Knative Serving v1 版的服务数据结构
package v1

import "github.com/openfaas/faas-cli/schema"

// APIVersionLatest 最新的 Knative API 版本
const APIVersionLatest = "serving.knative.dev/v1"

// ServingServiceCRD 服务的顶层 YAML 结构
type ServingServiceCRD struct {
	APIVersion string             `yaml:"apiVersion"`         // API 版本
	Kind       string             `yaml:"kind"`               // 资源类型
	Metadata   schema.Metadata    `yaml:"metadata,omitempty"` // 元数据
	Spec       ServingServiceSpec `yaml:"spec"`               // 服务规格
}

// ServingServiceSpec 服务规格主体
type ServingServiceSpec struct {
	ServingServiceSpecTemplate `yaml:"template"`
}

// ServingServiceSpecTemplateSpec Pod 规格配置
type ServingServiceSpecTemplateSpec struct {
	Containers []ServingSpecContainersContainerSpec `yaml:"containers"`        // 容器列表
	Volumes    []Volume                             `yaml:"volumes,omitempty"` // 数据卷
}

// ServingServiceSpecTemplate 模板封装
type ServingServiceSpecTemplate struct {
	Template ServingServiceSpecTemplateSpec `yaml:"spec"`
}

// ServingSpecContainersContainerSpec 容器配置
type ServingSpecContainersContainerSpec struct {
	Image        string        `yaml:"image"`                  // 容器镜像
	Env          []EnvPair     `yaml:"env,omitempty"`          // 环境变量
	VolumeMounts []VolumeMount `yaml:"volumeMounts,omitempty"` // 卷挂载
}

// VolumeMount 卷挂载信息
type VolumeMount struct {
	Name      string `yaml:"name"`      // 挂载名称
	MountPath string `yaml:"mountPath"` // 容器内路径
	ReadOnly  bool   `yaml:"readOnly"`  // 是否只读
}

// Volume 数据卷定义
type Volume struct {
	Name   string `yaml:"name"`   // 卷名称
	Secret Secret `yaml:"secret"` // 密钥卷
}

// Secret 密钥资源引用
type Secret struct {
	SecretName string `yaml:"secretName"` // 密钥名称
}

// EnvPair 环境变量键值对
type EnvPair struct {
	Name  string `yaml:"name"`  // 变量名
	Value string `yaml:"value"` // 变量值
}
