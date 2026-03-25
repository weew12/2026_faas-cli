// Copyright (c) OpenFaaS Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 secret create 命令，用于创建函数密钥
package commands

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/openfaas/faas-cli/proxy"
	types "github.com/openfaas/faas-provider/types"
	"github.com/spf13/cobra"
)

// 命令行参数
var (
	literalSecret string // 从字面量读取密钥值
	secretFile    string // 从文件读取密钥值
	trimSecret    bool   // 是否去除密钥首尾空白
	replaceSecret bool   // 存在则更新（替代创建）
)

// secretCreateCmd 创建新的函数密钥
// 支持从字面量、文件、标准输入读取值，可自动替换已存在密钥
var secretCreateCmd = &cobra.Command{
	Use: `create SECRET_NAME
			[--trim=false]
			[--from-literal=SECRET_VALUE]
			[--from-file=/path/to/secret/file]
			[--replace]
			[STDIN]
			[--tls-no-verify]`,
	Short: "Create a new secret",
	Long:  `The create command creates a new secret from file, literal or STDIN`,
	Example: `  # Create a secret from a literal value in the default namespace
  faas-cli secret create NAME --from-literal=VALUE

  # Create a secret in a specific namespace
  faas-cli secret create NAME --from-literal=VALUE \
    --namespace=NS

  # Create the secret from a file with a given gateway
  faas-cli secret create NAME --from-file=PATH \
    --gateway=http://127.0.0.1:8080

  # Create the secret from a STDIN pipe
  cat ./secret.txt | faas-cli secret create NAME
  
  # Force an update if the secret already exists
  faas-cli secret create NAME --from-file PATH --replace
`,
	RunE:    runSecretCreate,
	PreRunE: preRunSecretCreate,
}

// init 初始化命令参数并注册到 secret 根命令
func init() {
	secretCreateCmd.Flags().StringVar(&literalSecret, "from-literal", "", "Literal value for the secret")
	secretCreateCmd.Flags().StringVar(&secretFile, "from-file", "", "Path and filename containing value for the secret")
	secretCreateCmd.Flags().BoolVar(&trimSecret, "trim", true, "Trim whitespace from the start and end of the secret value")
	secretCreateCmd.Flags().BoolVar(&tlsInsecure, "tls-no-verify", false, "Disable TLS validation")
	secretCreateCmd.Flags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL starting with http(s)://")
	secretCreateCmd.Flags().StringVarP(&token, "token", "k", "", "Pass a JWT token to use instead of basic auth")
	secretCreateCmd.Flags().StringVarP(&functionNamespace, "namespace", "n", "", "Namespace of the function")
	secretCreateCmd.Flags().BoolVar(&replaceSecret, "replace", false, "Replace the secret if it already exists using an update")

	secretCmd.AddCommand(secretCreateCmd)
}

// preRunSecretCreate 执行前参数校验
// 1. 必须提供密钥名称
// 2. 只能使用一种密钥输入方式
// 3. 验证密钥名称符合 DNS-1123 规范
func preRunSecretCreate(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("secret name required")
	}

	if len(args) > 1 {
		return fmt.Errorf("too many values for secret name")
	}

	if len(secretFile) > 0 && len(literalSecret) > 0 {
		return fmt.Errorf("please provide secret using only one option from --from-literal, --from-file and STDIN")
	}

	isValid, err := validateSecretName(args[0])
	if !isValid {
		return err
	}

	return nil
}

// runSecretCreate 执行创建密钥的核心逻辑
// 1. 读取密钥值（literal/file/stdin）
// 2. 去除首尾空白（trim 开启时）
// 3. 创建 API 客户端
// 4. 创建密钥，冲突时根据 --replace 自动更新
func runSecretCreate(cmd *cobra.Command, args []string) error {
	secret := types.Secret{
		Name:      args[0],
		Namespace: functionNamespace,
	}

	// 从三种来源读取密钥值
	switch {
	case len(literalSecret) > 0:
		secret.Value = literalSecret

	case len(secretFile) > 0:
		fileData, err := os.ReadFile(secretFile)
		if err != nil {
			return err
		}

		secret.RawValue = fileData
		// 保留以兼容旧版
		secret.Value = string(fileData)

	default:
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			fmt.Fprintf(os.Stderr, "Reading from STDIN - hit (Control + D) to stop.\n")
		}

		secretStdin, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		secret.Value = string(secretStdin)
	}

	// 去除首尾空白
	if trimSecret {
		secret.Value = strings.TrimSpace(secret.Value)
	}

	// 密钥值不能为空
	if len(secret.Value) == 0 {
		return fmt.Errorf("must provide a non empty secret via --from-literal, --from-file or STDIN")
	}

	gatewayAddress := getGatewayURL(gateway, defaultGateway, "", os.Getenv(openFaaSURLEnvironment))

	if msg := checkTLSInsecure(gatewayAddress, tlsInsecure); len(msg) > 0 {
		fmt.Println(msg)
	}
	cliAuth, err := proxy.NewCLIAuth(token, gatewayAddress)
	if err != nil {
		return err
	}
	transport := GetDefaultCLITransport(tlsInsecure, &commandTimeout)
	client, err := proxy.NewClient(cliAuth, gatewayAddress, transport, &commandTimeout)
	if err != nil {
		return err
	}

	fmt.Printf("Creating secret: %s.%s\n", secret.Name, functionNamespace)
	status, output := client.CreateSecret(context.Background(), secret)

	// 已存在且开启 replace 则自动更新
	if status == http.StatusConflict && replaceSecret {
		fmt.Printf("Secret %s already exists, updating...\n", secret.Name)
		_, output = client.UpdateSecret(context.Background(), secret)
	}

	fmt.Print(output)

	return nil
}

// Kubernetes DNS-1123 子域名正则规则
const (
	dns1123LabelFmt          string = "[a-z0-9]([-a-z0-9]*[a-z0-9])?"
	dns1123SubdomainFmt      string = dns1123LabelFmt + "(\\." + dns1123LabelFmt + ")*"
	invalidSecretNameMessage string = "ERROR: invalid secret name %s\nSecret name must start and end with an alphanumeric character \nand can only contain lower-case alphanumeric characters, '-' or '.'"
)

// validateSecretName 验证密钥名称符合 Kubernetes DNS-1123 命名规范
func validateSecretName(secretName string) (bool, error) {
	var dns1123SubdomainRegexp = regexp.MustCompile("^" + dns1123SubdomainFmt + "$")

	if !dns1123SubdomainRegexp.MatchString(secretName) {
		return false, fmt.Errorf(invalidSecretNameMessage, secretName)
	}

	return true, nil
}
