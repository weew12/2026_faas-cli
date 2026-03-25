// Copyright (c) OpenFaaS Author(s) 2020. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 registry-login 命令，用于生成并保存容器镜像仓库的认证配置文件
package commands

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// registryLoginCommand 生成容器镜像仓库认证配置文件
// 支持标准 Docker 仓库和 AWS ECR，认证信息会保存为 ./credentials/config.json
var registryLoginCommand = &cobra.Command{
	Use:          "registry-login",
	Short:        "Generate and save the registry authentication file",
	SilenceUsage: true,
	RunE:         generateRegistryAuthFile,
	PreRunE:      generateRegistryPreRun,
}

// init 初始化命令参数并注册到主命令 faasCmd
func init() {
	registryLoginCommand.Flags().String("server", "https://index.docker.io/v1/", "The server URL, it is defaulted to the docker registry")
	registryLoginCommand.Flags().StringP("username", "u", "", "The Registry Username")
	registryLoginCommand.Flags().String("password", "", "The registry password")
	registryLoginCommand.Flags().BoolP("password-stdin", "s", false, "Reads the docker password from stdin, either pipe to the command or remember to press ctrl+d when reading interactively")

	registryLoginCommand.Flags().Bool("ecr", false, "If we are using ECR we need a different set of flags, so if this is set, we need to set --account-id and --region")
	registryLoginCommand.Flags().String("account-id", "", "Your AWS Account id")
	registryLoginCommand.Flags().String("region", "", "Your AWS region")

	faasCmd.AddCommand(registryLoginCommand)
}

// generateRegistryPreRun 命令执行前的参数校验
// 验证所有标志位是否合法，ECR 模式下必须传入 account-id 和 region
func generateRegistryPreRun(command *cobra.Command, args []string) error {
	_, err := command.Flags().GetString("server")
	if err != nil {
		return fmt.Errorf("error with --server usage: %s", err)
	}

	_, err = command.Flags().GetString("username")
	if err != nil {
		return fmt.Errorf("error with --username usage: %s", err)
	}

	_, err = command.Flags().GetString("password")
	if err != nil {
		return fmt.Errorf("error with --password usage: %s", err)
	}

	_, err = command.Flags().GetBool("password-stdin")
	if err != nil {
		return fmt.Errorf("error with --password-stdin usage: %s", err)
	}

	ecr, err := command.Flags().GetBool("ecr")
	if err != nil {
		return fmt.Errorf("error with --ecr usage: %s", err)
	}

	accountID, err := command.Flags().GetString("account-id")
	if err != nil {
		return fmt.Errorf("error with --account-id usage: %s", err)
	}

	region, err := command.Flags().GetString("region")
	if err != nil {
		return fmt.Errorf("error with --region usage: %s", err)
	}

	if ecr {
		if len(accountID) == 0 {
			return fmt.Errorf("the --account-id flag is required with ECR")
		}
		if len(region) == 0 {
			return fmt.Errorf("the --region flag is required with ECR")
		}
	}

	return nil
}

// generateRegistryAuthFile 执行生成认证文件的核心逻辑
// 分支处理：标准仓库 / ECR 仓库 / 密码从标准输入读取
func generateRegistryAuthFile(command *cobra.Command, _ []string) error {
	ecrEnabled, _ := command.Flags().GetBool("ecr")
	accountID, _ := command.Flags().GetString("account-id")
	region, _ := command.Flags().GetString("region")
	username, _ := command.Flags().GetString("username")
	password, _ := command.Flags().GetString("password")
	server, _ := command.Flags().GetString("server")
	passStdin, _ := command.Flags().GetBool("password-stdin")

	if ecrEnabled {
		if err := generateECRFile(accountID, region); err != nil {
			return err
		}

	} else if passStdin {
		fmt.Printf("Enter your password, hit enter then type Ctrl+D\n\nPassword: ")
		passwordStdin, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		if err := generateFile(username, strings.TrimSpace(string(passwordStdin)), server); err != nil {
			return err
		}
	} else {
		if err := generateFile(username, password, server); err != nil {
			return err
		}
	}

	fmt.Printf("\nWrote ./credentials/config.json..OK\n")

	return nil
}

// generateFile 生成普通镜像仓库认证配置并写入文件
func generateFile(username string, password string, server string) error {
	fileBytes, err := generateRegistryAuth(server, username, password)
	if err != nil {
		return err
	}
	return writeFileToFassCLITmp(fileBytes)
}

// generateECRFile 生成 AWS ECR 镜像仓库认证配置并写入文件
func generateECRFile(accountID string, region string) error {
	fileBytes, err := generateECRRegistryAuth(accountID, region)
	if err != nil {
		return err
	}

	return writeFileToFassCLITmp(fileBytes)
}

// generateRegistryAuth 构造标准 Docker 认证配置（Base64 编码）
func generateRegistryAuth(server, username, password string) ([]byte, error) {
	if len(username) == 0 || len(password) == 0 || len(server) == 0 {
		return nil, errors.New("both --username and (--password-stdin or --password) are required")
	}

	encodedString := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
	data := RegistryAuth{
		AuthConfigs: map[string]Auth{
			server: {Base64AuthString: encodedString},
		},
	}

	registryBytes, err := json.MarshalIndent(data, "", " ")

	return registryBytes, err
}

// generateECRRegistryAuth 构造 ECR 认证配置（使用 credsStore 辅助工具）
func generateECRRegistryAuth(accountID, region string) ([]byte, error) {
	if len(accountID) == 0 || len(region) == 0 {
		return nil, errors.New("you must provide an --account-id and --region when using --ecr")
	}

	data := ECRRegistryAuth{
		CredsStore: "ecr-login",
		CredHelpers: map[string]string{
			fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", accountID, region): "ecr-login",
		},
	}

	registryBytes, err := json.MarshalIndent(data, "", " ")

	return registryBytes, err
}

// writeFileToFassCLITmp 将配置写入 ./credentials/config.json
func writeFileToFassCLITmp(fileBytes []byte) error {
	path := "./credentials"
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		err := os.Mkdir(path, 0744)
		if err != nil {
			return err
		}
	}

	return os.WriteFile(filepath.Join(path, "config.json"), fileBytes, 0744)
}

// Auth 仓库认证结构（存储 Base64 编码的用户名密码）
type Auth struct {
	Base64AuthString string `json:"auth"`
}

// RegistryAuth Docker 配置文件根结构
type RegistryAuth struct {
	AuthConfigs map[string]Auth `json:"auths"`
}

// ECRRegistryAuth ECR 配置文件结构（使用凭证助手）
type ECRRegistryAuth struct {
	CredsStore  string            `json:"credsStore"`
	CredHelpers map[string]string `json:"credHelpers"`
}
