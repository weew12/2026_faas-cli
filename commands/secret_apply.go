// Copyright (c) OpenFaaS Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 secret apply 命令，用于批量同步本地 .secrets 目录下的密钥到网关
package commands

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/openfaas/faas-cli/proxy"
	types "github.com/openfaas/faas-provider/types"
	"github.com/spf13/cobra"
)

// secretApplyCmd 批量应用本地 .secrets 目录中的所有密钥
// 会自动同步文件名为密钥名，覆盖网关中已存在的同名密钥
var secretApplyCmd = &cobra.Command{
	Use:   `apply [--tls-no-verify]`,
	Short: "Apply secrets from .secrets folder",
	Long:  `Apply all secrets from the .secrets folder to the gateway. Each file in .secrets/ will be synced to the gateway, replacing existing secrets with the same name.`,
	Example: `  # Apply all secrets from .secrets folder
  faas-cli secret apply
  
  # Apply secrets to a specific namespace
  faas-cli secret apply --namespace=my-namespace
  
  # Apply secrets with a custom gateway
  faas-cli secret apply --gateway=http://127.0.0.1:8080`,
	RunE: runSecretApply,
}

// init 初始化命令参数并注册到 secret 根命令
func init() {
	secretApplyCmd.Flags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL starting with http(s)://")
	secretApplyCmd.Flags().BoolVar(&tlsInsecure, "tls-no-verify", false, "Disable TLS validation")
	secretApplyCmd.Flags().StringVarP(&token, "token", "k", "", "Pass a JWT token to use instead of basic auth")
	secretApplyCmd.Flags().StringVarP(&functionNamespace, "namespace", "n", "", "Namespace of the function")
	secretApplyCmd.Flags().BoolVar(&trimSecret, "trim", true, "Trim whitespace from the start and end of the secret value")

	secretCmd.AddCommand(secretApplyCmd)
}

// runSecretApply 执行批量同步密钥的核心逻辑
// 1. 连接到 OpenFaaS 网关
// 2. 检查本地 .secrets 目录是否存在
// 3. 读取目录下所有文件作为密钥
// 4. 获取网关现有密钥用于对比
// 5. 逐个创建/覆盖密钥
func runSecretApply(cmd *cobra.Command, args []string) error {
	gatewayAddress := getGatewayURL(gateway, defaultGateway, "", os.Getenv(openFaaSURLEnvironment))

	if msg := checkTLSInsecure(gatewayAddress, tlsInsecure); len(msg) > 0 {
		fmt.Println(msg)
	}

	// 创建 OpenFaaS 客户端
	cliAuth, err := proxy.NewCLIAuth(token, gatewayAddress)
	if err != nil {
		return err
	}
	transport := GetDefaultCLITransport(tlsInsecure, &commandTimeout)
	client, err := proxy.NewClient(cliAuth, gatewayAddress, transport, &commandTimeout)
	if err != nil {
		return err
	}

	// 获取 .secrets 目录的绝对路径
	secretsPath, err := filepath.Abs(localSecretsDir)
	if err != nil {
		return fmt.Errorf("can't determine secrets folder: %w", err)
	}

	// 检查 .secrets 目录是否存在
	if _, err := os.Stat(secretsPath); os.IsNotExist(err) {
		return fmt.Errorf("secrets directory does not exist: %s", secretsPath)
	}

	// 读取目录下所有文件
	files, err := os.ReadDir(secretsPath)
	if err != nil {
		return fmt.Errorf("failed to read secrets directory: %w", err)
	}

	// 无密钥时直接返回
	if len(files) == 0 {
		fmt.Println("No secrets found in .secrets directory")
		return nil
	}

	// 获取网关已存在的密钥列表
	existingSecrets, err := client.GetSecretList(context.Background(), functionNamespace)
	if err != nil {
		return fmt.Errorf("failed to get secret list: %w", err)
	}

	// 构建已存在密钥的快速查询 map
	secretMap := make(map[string]bool)
	for _, secret := range existingSecrets {
		if secret.Namespace == functionNamespace {
			secretMap[secret.Name] = true
		}
	}

	// 遍历处理每个密钥文件
	for _, file := range files {
		// 跳过子目录
		if file.IsDir() {
			continue
		}

		secretName := file.Name()

		// 验证密钥名称是否合法
		isValid, err := validateSecretName(secretName)
		if !isValid {
			fmt.Printf("Skipping invalid secret name: %s - %v\n", secretName, err)
			continue
		}

		// 读取密钥文件内容
		secretFilePath := filepath.Join(secretsPath, secretName)
		fileData, err := os.ReadFile(secretFilePath)
		if err != nil {
			fmt.Printf("Failed to read secret file %s: %v\n", secretName, err)
			continue
		}

		// 处理空白字符
		secretValue := string(fileData)
		if trimSecret {
			secretValue = strings.TrimSpace(secretValue)
		}

		// 跳过空密钥
		if len(secretValue) == 0 {
			fmt.Printf("Skipping empty secret: %s\n", secretName)
			continue
		}

		// 构造密钥对象
		secret := types.Secret{
			Name:      secretName,
			Namespace: functionNamespace,
			Value:     secretValue,
			RawValue:  fileData,
		}

		// 如果密钥已存在，先删除
		if secretMap[secretName] {
			fmt.Printf("Secret %s exists, deleting before recreating...\n", secretName)
			deleteSecret := types.Secret{
				Name:      secretName,
				Namespace: functionNamespace,
			}
			err = client.RemoveSecret(context.Background(), deleteSecret)
			if err != nil {
				fmt.Printf("Failed to remove existing secret %s: %v\n", secretName, err)
			}
		}

		// 创建密钥
		fmt.Printf("Creating secret: %s.%s\n", secret.Name, functionNamespace)
		status, output := client.CreateSecret(context.Background(), secret)

		// 冲突时降级为更新操作
		if status == http.StatusConflict {
			fmt.Printf("Secret %s still exists, updating...\n", secretName)
			_, output = client.UpdateSecret(context.Background(), secret)
		}

		fmt.Print(output)
	}

	return nil
}
