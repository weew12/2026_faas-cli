// Copyright (c) OpenFaaS Author(s) 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package config 提供 OpenFaaS CLI 的配置管理功能
// 包含配置文件读写、认证信息存储/查询/删除、路径管理、编解码等核心能力
// 配置默认存储于 ~/.openfaas/config.yml，支持环境变量自定义路径
package config

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"gopkg.in/yaml.v3"
)

// AuthType 认证类型字符串定义
type AuthType string

const (
	// BasicAuthType 基础认证类型（用户名+密码）
	BasicAuthType = "basic"
	// Oauth2AuthType OAuth2 认证类型
	Oauth2AuthType = "oauth2"

	// ConfigLocationEnv 用于覆盖默认配置目录的环境变量名称
	ConfigLocationEnv string = "OPENFAAS_CONFIG"

	// DefaultDir 默认配置存储目录（用户主目录下）
	DefaultDir string = "~/.openfaas"
	// DefaultFile 默认配置文件名
	DefaultFile string = "config.yml"
	// DefaultPermissions 默认配置目录权限（仅当前用户可读写）
	DefaultPermissions os.FileMode = 0700

	// DefaultCIDir CI 环境下的配置目录（当前工作目录）
	DefaultCIDir string = ".openfaas"
	// DefaultCIPermissions CI 环境下的配置目录权限（允许其他用户读取）
	DefaultCIPermissions os.FileMode = 0744
)

// ConfigFile OpenFaaS CLI 配置文件顶层结构
type ConfigFile struct {
	AuthConfigs []AuthConfig `yaml:"auths"` // 认证配置列表
	FilePath    string       `yaml:"-"`     // 配置文件在磁盘上的路径（不序列化到YAML）
}

// AuthConfig 网关认证配置项
type AuthConfig struct {
	Gateway string   `yaml:"gateway,omitempty"` // OpenFaaS 网关地址
	Auth    AuthType `yaml:"auth,omitempty"`    // 认证类型
	Token   string   `yaml:"token,omitempty"`   // 认证令牌/凭证
	Options []Option `yaml:"options,omitempty"` // 扩展配置项
}

// Option 键值对形式的扩展配置
type Option struct {
	Name  string `yaml:"name"`  // 配置项名称
	Value string `yaml:"value"` // 配置项值
}

// ErrConfigNotFound 配置文件不存在错误
var ErrConfigNotFound = errors.New("config file not found")

// AuthConfigNotFoundError 未找到指定网关的认证配置错误
type AuthConfigNotFoundError struct {
	Gateway string // 未找到认证的网关地址
}

// Error 实现 error 接口，返回格式化的错误信息
func (e *AuthConfigNotFoundError) Error() string {
	return fmt.Sprintf("no auth config found for %s", e.Gateway)
}

// New 初始化一个配置文件实例
// 参数 filePath：配置文件在磁盘上的完整路径
// 返回：配置实例或创建失败的错误
func New(filePath string) (*ConfigFile, error) {
	if filePath == "" {
		return nil, fmt.Errorf("can't create config with empty filePath")
	}
	conf := &ConfigFile{
		AuthConfigs: make([]AuthConfig, 0),
		FilePath:    filePath,
	}

	return conf, nil
}

// ConfigDir 获取配置文件存储目录路径
// 优先级：环境变量OPENFAAS_CONFIG > CI环境默认路径 > 主目录默认路径
func ConfigDir() string {
	override := os.Getenv(ConfigLocationEnv)
	ci := isRunningInCI()

	switch {
	// CI环境且未设置自定义路径
	case ci && override == "":
		return DefaultCIDir
	// 设置了自定义路径
	case override != "":
		return override
	// 常规环境使用默认路径
	default:
		return DefaultDir
	}
}

// isRunningInCI 判断当前是否运行在CI环境中
// 通过检查环境变量 CI=true 或 CI=1 进行判断
func isRunningInCI() bool {
	if env, ok := os.LookupEnv("CI"); ok {
		if env == "true" || env == "1" {
			return true
		}
	}
	return false
}

// EnsureFile 确保配置目录和文件存在，不存在则自动创建
// 返回：配置文件完整路径 或 创建失败的错误
func EnsureFile() (string, error) {
	permission := DefaultPermissions
	dir := ConfigDir()
	if isRunningInCI() {
		permission = DefaultCIPermissions
	}
	dirPath, err := homedir.Expand(dir)
	if err != nil {
		return "", err
	}

	filePath := path.Clean(filepath.Join(dirPath, DefaultFile))
	if err := os.MkdirAll(filepath.Dir(filePath), permission); err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("error creating directory: %s - %w", filepath.Dir(filePath), err)
	}

	// 文件不存在则创建空文件
	if _, err := os.Stat(filePath); err != nil && os.IsNotExist(err) {
		file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return "", err
		}
		defer file.Close()
	}

	return filePath, nil
}

// fileExists 判断默认路径下的配置文件是否存在
func fileExists() bool {
	dir := ConfigDir()
	dirPath, err := homedir.Expand(dir)
	if err != nil {
		return false
	}

	filePath := path.Clean(filepath.Join(dirPath, DefaultFile))
	if _, err := os.Stat(filePath); err != nil && os.IsNotExist(err) {
		return false
	}

	return true
}

// save 将配置写入磁盘文件
// 方法接收者：配置文件实例
func (configFile *ConfigFile) save() error {
	file, err := os.OpenFile(configFile.FilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	var buff bytes.Buffer
	yamlEncoder := yaml.NewEncoder(&buff)
	yamlEncoder.SetIndent(2) // 设置YAML缩进为2
	if err := yamlEncoder.Encode(&configFile); err != nil {
		return err
	}

	_, err = file.Write(buff.Bytes())
	return err
}

// load 从磁盘读取并加载配置文件
// 方法接收者：配置文件实例
func (configFile *ConfigFile) load() error {
	conf := &ConfigFile{}

	if _, err := os.Stat(configFile.FilePath); err != nil && os.IsNotExist(err) {
		return fmt.Errorf("can't load config from non existent filePath")
	}

	data, err := os.ReadFile(configFile.FilePath)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, conf); err != nil {
		return err
	}

	// 仅覆盖认证配置，保留其他字段
	if len(conf.AuthConfigs) > 0 {
		configFile.AuthConfigs = conf.AuthConfigs
	}
	return nil
}

// EncodeAuth 将用户名和密码编码为Base64字符串
// 用于Basic认证的凭证存储
func EncodeAuth(username string, password string) string {
	input := username + ":" + password
	msg := []byte(input)
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(msg)))
	base64.StdEncoding.Encode(encoded, msg)
	return string(encoded)
}

// DecodeAuth 将Base64字符串解码为用户名和密码
// 返回：用户名、密码、解码失败的错误
func DecodeAuth(input string) (string, string, error) {
	decoded, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return "", "", err
	}
	arr := strings.SplitN(string(decoded), ":", 2)
	if len(arr) != 2 {
		return "", "", fmt.Errorf("invalid auth config file")
	}
	return arr[0], arr[1], nil
}

// UpdateAuthConfig 更新或新增网关认证配置
// 存在则覆盖，不存在则追加
func UpdateAuthConfig(authConfig AuthConfig) error {
	gateway := authConfig.Gateway

	// 校验网关地址格式
	_, err := url.ParseRequestURI(gateway)
	if err != nil || len(gateway) < 1 {
		return fmt.Errorf("invalid gateway URL")
	}

	configPath, err := EnsureFile()
	if err != nil {
		return err
	}

	cfg, err := New(configPath)
	if err != nil {
		return err
	}

	if err := cfg.load(); err != nil {
		return err
	}

	// 查找已存在的配置索引
	index := -1
	for i, v := range cfg.AuthConfigs {
		if gateway == v.Gateway {
			index = i
			break
		}
	}

	// 覆盖或追加配置
	if index == -1 {
		cfg.AuthConfigs = append(cfg.AuthConfigs, authConfig)
	} else {
		cfg.AuthConfigs[index] = authConfig
	}

	return cfg.save()
}

// LookupAuthConfig 根据网关地址查询认证配置
// 返回：查询到的认证配置 或 未找到错误
func LookupAuthConfig(gateway string) (AuthConfig, error) {
	var authConfig AuthConfig

	if !fileExists() {
		return authConfig, ErrConfigNotFound
	}

	configPath, err := EnsureFile()
	if err != nil {
		return authConfig, err
	}

	cfg, err := New(configPath)
	if err != nil {
		return authConfig, err
	}

	if err := cfg.load(); err != nil {
		return authConfig, err
	}

	// 遍历匹配网关地址
	for _, v := range cfg.AuthConfigs {
		if gateway == v.Gateway {
			authConfig = v
			return authConfig, nil
		}
	}

	return authConfig, &AuthConfigNotFoundError{Gateway: gateway}
}

// RemoveAuthConfig 根据网关地址删除认证配置
// 不存在则返回未找到错误
func RemoveAuthConfig(gateway string) error {
	if !fileExists() {
		return ErrConfigNotFound
	}

	configPath, err := EnsureFile()
	if err != nil {
		return err
	}

	cfg, err := New(configPath)
	if err != nil {
		return err
	}

	if err := cfg.load(); err != nil {
		return err
	}

	// 查找待删除配置索引
	index := -1
	for i, v := range cfg.AuthConfigs {
		if gateway == v.Gateway {
			index = i
			break
		}
	}

	if index > -1 {
		cfg.AuthConfigs = removeAuthByIndex(cfg.AuthConfigs, index)
		return cfg.save()
	}

	return &AuthConfigNotFoundError{Gateway: gateway}
}

// removeAuthByIndex 根据索引删除认证配置（内部工具方法）
// 切片原地删除，不保留原顺序
func removeAuthByIndex(s []AuthConfig, index int) []AuthConfig {
	return append(s[:index], s[index+1:]...)
}
