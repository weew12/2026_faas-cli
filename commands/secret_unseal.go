// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 secret unseal 命令，用于解密密封的 secrets 文件并查看内容
package commands

import (
	"fmt"
	"os"

	"github.com/openfaas/go-sdk/seal"
	"github.com/spf13/cobra"
)

// 命令行参数
var (
	unsealInput string // 密封的 secrets 文件路径
	unsealKey   string // 指定要解密的单个 key，不指定则解密全部
)

// secretUnsealCmd 解密密封的 secrets 文件
// 使用私钥解密并输出键值对，支持查看全部或单个密钥
var secretUnsealCmd = &cobra.Command{
	Use:   "unseal [private-key-file]",
	Short: "Unseal and inspect a sealed secrets file",
	Long:  "Decrypt a sealed secrets file using a private key and print the key/value pairs",
	Example: `  # Print all secrets
  faas-cli secret unseal key

  # Print a single secret value
  faas-cli secret unseal key --key pip_token

  # Specify a different sealed file
  faas-cli secret unseal key --in ./build/com.openfaas.secrets
`,
	Args: cobra.ExactArgs(1),
	RunE: runSecretUnseal,
}

// init 初始化命令，注册参数并添加到 secret 根命令
func init() {
	secretUnsealCmd.Flags().StringVar(&unsealInput, "in", "com.openfaas.secrets", "Path to the sealed secrets file")
	secretUnsealCmd.Flags().StringVar(&unsealKey, "key", "", "Unseal a single key (omit to print all)")

	secretCmd.AddCommand(secretUnsealCmd)
}

// runSecretUnseal 执行解密逻辑
// 1. 读取私钥
// 2. 读取密封的 secrets 文件
// 3. 根据参数解密全部或单个密钥
// 4. 输出结果
func runSecretUnseal(cmd *cobra.Command, args []string) error {
	// 读取私钥文件
	privKey, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("reading private key: %w", err)
	}

	// 读取密封的 secrets 文件
	envelope, err := os.ReadFile(unsealInput)
	if err != nil {
		return fmt.Errorf("reading sealed file: %w", err)
	}

	// 如果指定了 --key，则只解密单个密钥
	if unsealKey != "" {
		value, err := seal.UnsealKey(privKey, envelope, unsealKey)
		if err != nil {
			return err
		}
		fmt.Print(string(value))
		return nil
	}

	// 解密所有密钥
	values, err := seal.Unseal(privKey, envelope)
	if err != nil {
		return err
	}

	// 输出所有键值对
	for k, v := range values {
		fmt.Printf("%s=%s\n", k, string(v))
	}

	return nil
}
