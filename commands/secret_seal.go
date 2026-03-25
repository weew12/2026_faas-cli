// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 secret seal 命令，用于加密构建密钥并保存到加密文件
package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/openfaas/go-sdk/seal"
	"github.com/spf13/cobra"
)

// 命令行参数
var (
	sealOutput      string   // 加密后的输出文件路径
	sealFromLiteral []string // 从字面量读取密钥（key=value）
	sealFromFile    []string // 从文件读取密钥（key=path）
)

// secretSealCmd 使用公钥加密密钥并生成密封文件
// 支持从字面量或文件导入密钥，生成的加密文件可安全提交到Git或用于构建
var secretSealCmd = &cobra.Command{
	Use:   "seal [public-key-file]",
	Short: "Seal build secrets into an encrypted file",
	Long:  "Seal key/value pairs using a public key. The output file can be included in a build tar or committed to git.",
	Example: `  # Seal literal values
  faas-cli secret seal key.pub \
    --from-literal pip_token=s3cr3t \
    --from-literal npm_token=tok123

  # Seal from files (binary-safe)
  faas-cli secret seal key.pub \
    --from-file ca.crt=./certs/ca.crt \
    --from-literal api_key=sk-1234

  # Specify output path
  faas-cli secret seal key.pub \
    --from-literal token=s3cr3t \
    -o ./build/com.openfaas.secrets
`,
	Args:    cobra.ExactArgs(1),
	RunE:    runSecretSeal,
	PreRunE: preRunSecretSeal,
}

// init 初始化命令，注册参数并添加到 secret 根命令
func init() {
	secretSealCmd.Flags().StringVarP(&sealOutput, "output", "o", "com.openfaas.secrets", "Output file path")
	secretSealCmd.Flags().StringArrayVar(&sealFromLiteral, "from-literal", nil, "Literal secret in key=value format (can be repeated)")
	secretSealCmd.Flags().StringArrayVar(&sealFromFile, "from-file", nil, "Secret from file in key=path format (can be repeated)")

	secretCmd.AddCommand(secretSealCmd)
}

// preRunSecretSeal 执行前校验：必须提供至少一个密钥来源
func preRunSecretSeal(cmd *cobra.Command, args []string) error {
	if len(sealFromLiteral) == 0 && len(sealFromFile) == 0 {
		return fmt.Errorf("provide at least one secret via --from-literal or --from-file")
	}

	return nil
}

// runSecretSeal 执行密钥加密逻辑
// 1. 读取公钥
// 2. 解析并收集来自字面量和文件的密钥
// 3. 使用公钥加密所有密钥
// 4. 写入加密文件并输出结果
func runSecretSeal(cmd *cobra.Command, args []string) error {
	// 读取公钥文件
	pubKey, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("reading public key: %w", err)
	}

	values := make(map[string][]byte)

	// 解析 --from-literal 格式的密钥
	for _, lit := range sealFromLiteral {
		k, v, ok := strings.Cut(lit, "=")
		if !ok || k == "" {
			return fmt.Errorf("invalid --from-literal format %q, expected key=value", lit)
		}
		values[k] = []byte(v)
	}

	// 解析 --from-file 格式的密钥
	for _, f := range sealFromFile {
		k, path, ok := strings.Cut(f, "=")
		if !ok || k == "" || path == "" {
			return fmt.Errorf("invalid --from-file format %q, expected key=path", f)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading file for key %q: %w", k, err)
		}
		values[k] = data
	}

	// 执行加密
	sealed, err := seal.Seal(pubKey, values)
	if err != nil {
		return fmt.Errorf("sealing secrets: %w", err)
	}

	// 写入加密文件
	if err := os.WriteFile(sealOutput, sealed, 0600); err != nil {
		return fmt.Errorf("writing sealed file: %w", err)
	}

	// 输出成功信息
	keyID, _ := seal.DeriveKeyID(pubKey)
	fmt.Printf("Sealed %d secret(s) to %s (key ID: %s)\n", len(values), sealOutput, keyID)

	return nil
}
