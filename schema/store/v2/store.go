// Copyright (c) OpenFaaS Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package v2

// StoreFunction 表示应用商店中的一个多架构函数
type StoreFunction struct {
	Icon                   string            `json:"icon"`
	Author                 string            `json:"author,omitempty"`
	Title                  string            `json:"title"`
	Description            string            `json:"description"`
	Name                   string            `json:"name"`
	Fprocess               string            `json:"fprocess"`
	RepoURL                string            `json:"repo_url"`
	ReadOnlyRootFilesystem bool              `json:"readOnlyRootFilesystem"`
	Environment            map[string]string `json:"environment"`
	Labels                 map[string]string `json:"labels"`
	Annotations            map[string]string `json:"annotations"`
	Images                 map[string]string `json:"images"`
}

// GetImageName 根据平台名称获取对应的函数镜像名称
func (s *StoreFunction) GetImageName(platform string) string {
	imageName, _ := s.Images[platform]
	return imageName
}

// Store 表示 v2 版本的应用商店列表结构
type Store struct {
	Version   string          `json:"version"`
	Functions []StoreFunction `json:"functions"`
}
