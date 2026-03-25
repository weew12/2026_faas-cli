// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 secret generate 命令，用于生成加密安全的随机密钥
package commands

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// 命令行参数
var (
	generateLength int    // 生成随机字节的长度
	generateOutput string // 输出文件路径（为空则输出到标准输出）
)

// secretGenerateCmd 生成加密安全的随机密钥
// 输出为 Base64 编码，适用于 HMAC 签名、共享密钥等场景
var secretGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a random secret value",
	Long:  "Generate a cryptographically random secret suitable for HMAC payload signing or other shared secrets",
	Example: `  # Print a 32-byte base64-encoded secret to stdout
  faas-cli secret generate

  # Write to a file
  faas-cli secret generate -o payload.txt

  # Custom length in bytes
  faas-cli secret generate --length 64
`,
	RunE: runSecretGenerate,
}

// init 初始化命令参数并注册到 secret 根命令
func init() {
	secretGenerateCmd.Flags().IntVar(&generateLength, "length", 32, "Number of random bytes")
	secretGenerateCmd.Flags().StringVarP(&generateOutput, "output", "o", "", "Write to file instead of stdout")

	secretCmd.AddCommand(secretGenerateCmd)
}

// runSecretGenerate 执行随机密钥生成逻辑
// 1. 从 crypto/rand 生成安全随机数
// 2. 编码为 Base64 字符串
// 3. 输出到文件或标准输出
func runSecretGenerate(cmd *cobra.Command, args []string) error {
	// 生成指定长度的安全随机字节
	buf := make([]byte, generateLength)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("generating random bytes: %w", err)
	}

	// 编码为标准 Base64 格式
	secret := base64.StdEncoding.EncodeToString(buf)

	// 输出到文件
	if generateOutput != "" {
		dir := filepath.Dir(generateOutput)
		// 自动创建目录
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0700); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}
		}

		// 写入文件并设置安全权限
		if err := os.WriteFile(generateOutput, []byte(secret), 0600); err != nil {
			return fmt.Errorf("writing secret: %w", err)
		}
		fmt.Printf("Wrote %d-byte secret to %s\n", generateLength, generateOutput)
	} else {
		// 直接打印到控制台
		fmt.Println(secret)
	}

	return nil
}
