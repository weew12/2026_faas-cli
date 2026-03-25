// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 secret keygen 命令，用于生成密钥对以加密构建密钥
package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/openfaas/go-sdk/seal"
	"github.com/spf13/cobra"
)

// keygenOutput 私钥输出路径（公钥自动追加 .pub）
var keygenOutput string

// secretKeygenCmd 生成 Curve25519 密钥对
// 用于 secrets 加密密封（seal/unseal），配合 pro-builder 使用
var secretKeygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate a keypair for sealing build secrets",
	Long:  "Generate a Curve25519 keypair for use with faas-cli secret seal and the pro-builder",
	Example: `  # Generate key and key.pub in the current directory
  faas-cli secret keygen

  # Generate mykey and mykey.pub in a specific directory
  faas-cli secret keygen -o ./keys/mykey
`,
	RunE: runSecretKeygen,
}

// init 初始化命令参数并注册到 secret 根命令
func init() {
	secretKeygenCmd.Flags().StringVarP(&keygenOutput, "output", "o", "key", "Output path for the private key (public key gets .pub appended)")

	secretCmd.AddCommand(secretKeygenCmd)
}

// runSecretKeygen 执行密钥对生成逻辑
// 1. 生成 Curve25519 公钥/私钥
// 2. 确保输出目录存在
// 3. 写入私钥（权限 0600）
// 4. 写入公钥（权限 0644）
// 5. 计算并输出 Key ID
func runSecretKeygen(cmd *cobra.Command, args []string) error {
	// 生成 Curve25519 密钥对
	pub, priv, err := seal.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("generating keypair: %w", err)
	}

	// 公钥/私钥文件路径
	privPath := keygenOutput
	pubPath := keygenOutput + ".pub"

	// 创建输出目录
	dir := filepath.Dir(privPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	// 写入私钥（严格权限）
	if err := os.WriteFile(privPath, priv, 0600); err != nil {
		return fmt.Errorf("writing private key: %w", err)
	}

	// 写入公钥
	if err := os.WriteFile(pubPath, pub, 0644); err != nil {
		return fmt.Errorf("writing public key: %w", err)
	}

	// 生成密钥标识
	keyID, err := seal.DeriveKeyID(pub)
	if err != nil {
		return fmt.Errorf("deriving key ID: %w", err)
	}

	// 输出结果
	fmt.Printf("Wrote private key: %s\n", privPath)
	fmt.Printf("Wrote public key:  %s\n", pubPath)
	fmt.Printf("Key ID:            %s\n", keyID)

	return nil
}
