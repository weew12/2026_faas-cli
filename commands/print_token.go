// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	// 注册 print-token 命令到主命令
	faasCmd.AddCommand(printToken)
}

// printToken 命令：格式化打印 JWT Token 内容（调试用，命令被隐藏）
var printToken = &cobra.Command{
	Use:   `print-token ./token.txt`,
	Short: "Pretty-print the contents of a JWT token",
	Example: `  # Print the contents of a JWT token
  faas-cli print-token ./token.txt
`,
	RunE:   runPrintTokenE,
	Hidden: true, // 隐藏命令，不在 help 中显示
}

// runPrintTokenE 执行 print-token 命令的主逻辑
func runPrintTokenE(cmd *cobra.Command, args []string) error {

	// 必须传入 token 文件路径
	if len(args) < 1 {
		return fmt.Errorf("provide the filename as an argument i.e. faas-cli print-token ./token.txt")
	}

	tokenFile := args[0]

	// 读取 token 文件内容
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return err
	}

	token := string(data)

	// 解析 JWT token
	jwtToken, err := unmarshalJwt(token)
	if err != nil {
		return err
	}

	// 格式化输出 JSON
	j, err := json.MarshalIndent(jwtToken, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(j))

	return nil
}

// JwtToken 存储解析后的 JWT 头部和负载
type JwtToken struct {
	Header  map[string]interface{} `json:"header"`
	Payload map[string]interface{} `json:"payload"`
}

// unmarshalJwt 解析 JWT token，解码 Base64 并反序列化为结构体
func unmarshalJwt(token string) (JwtToken, error) {

	// JWT 格式：header.payload.signature
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return JwtToken{}, fmt.Errorf("token should have 3 parts, got %d", len(parts))
	}

	// 解码 Header（RawURLEncoding 是 JWT 标准编码）
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return JwtToken{}, err
	}

	// 解码 Payload
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return JwtToken{}, err
	}

	var jwt JwtToken

	// 反序列化 Header
	err = json.Unmarshal(header, &jwt.Header)
	if err != nil {
		return JwtToken{}, err
	}

	// 反序列化 Payload
	err = json.Unmarshal(payload, &jwt.Payload)
	if err != nil {
		return JwtToken{}, err
	}

	return jwt, nil
}
