// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"bytes"
	"crypto"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"

	"github.com/alexellis/hmac/v2"
	"github.com/openfaas/faas-cli/version"
	"github.com/openfaas/go-sdk/stack"
	"github.com/spf13/cobra"
)

// 全局命令行参数
var (
	contentType             string   // 请求 Content-Type
	query                   []string // URL 查询参数
	headers                 []string // HTTP 请求头
	invokeAsync             bool     // 是否异步调用
	httpMethod              string   // HTTP 方法 (GET/POST/PUT等)
	sigHeader               string   // 签名请求头名称
	key                     string   // 签名密钥
	functionInvokeNamespace string   // 函数命名空间
	authenticate            bool     // 是否启用认证
)

// functionInvokeRealm 函数调用的认证领域标识
const functionInvokeRealm = "IAM function invoke"

func init() {
	// 配置命令行标志
	invokeCmd.Flags().StringVar(&functionName, "name", "", "Name of the deployed function")
	invokeCmd.Flags().StringVarP(&functionInvokeNamespace, "namespace", "n", "", "Namespace of the deployed function")

	invokeCmd.Flags().StringVarP(&gateway, "gateway", "g", defaultGateway, "Gateway URL starting with http(s)://")

	invokeCmd.Flags().StringVar(&contentType, "content-type", "text/plain", "The content-type HTTP header such as application/json")
	invokeCmd.Flags().StringArrayVar(&query, "query", []string{}, "pass query-string options")
	invokeCmd.Flags().StringArrayVarP(&headers, "header", "H", []string{}, "pass HTTP request header")
	invokeCmd.Flags().BoolVar(&authenticate, "auth", false, "Authenticate with an OpenFaaS token when invoking the function")
	invokeCmd.Flags().BoolVarP(&invokeAsync, "async", "a", false, "Invoke the function asynchronously")
	invokeCmd.Flags().StringVarP(&httpMethod, "method", "m", "POST", "pass HTTP request method")
	invokeCmd.Flags().BoolVar(&tlsInsecure, "tls-no-verify", false, "Disable TLS validation")
	invokeCmd.Flags().StringVar(&sigHeader, "sign", "", "name of HTTP request header to hold the signature")
	invokeCmd.Flags().StringVar(&key, "key", "", "key to be used to sign the request (must be used with --sign)")

	invokeCmd.Flags().BoolVar(&envsubst, "envsubst", true, "Substitute environment variables in stack.yaml file")

	// 注册到主命令
	faasCmd.AddCommand(invokeCmd)
}

// invokeCmd 调用已部署的 OpenFaaS 函数
var invokeCmd = &cobra.Command{
	Use:   `invoke FUNCTION_NAME [--gateway GATEWAY_URL] [--content-type CONTENT_TYPE] [--query KEY=VALUE] [--header "KEY: VALUE"] [--method HTTP_METHOD]`,
	Short: "Invoke an OpenFaaS function",
	Long:  `Invokes an OpenFaaS function and reads from STDIN for the body of the request`,
	Example: `  faas-cli invoke printer --gateway https://host:port <<< "Hello"
  faas-cli invoke echo --gateway https://host:port --content-type application/json
  faas-cli invoke env --query repo=faas-cli --query org=openfaas
  faas-cli invoke env --header "X-Ping-Url: http://request.bin/etc"
  faas-cli invoke resize-img --async -H "X-Callback-Url: http://gateway:8080/function/send2slack" < image.png
  faas-cli invoke env -H X-Ping-Url: http://request.bin/etc
  faas-cli invoke flask --method GET --namespace dev
  faas-cli invoke env --sign X-GitHub-Event --key yoursecret`,
	RunE: runInvoke,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// 执行前解析 yaml 文件
		if len(yamlFile) > 0 {
			parsedServices, err := stack.ParseYAMLFile(yamlFile, regex, filter, envsubst)
			if err != nil {
				return err
			}
			services = parsedServices
		}

		return nil
	},
}

// runInvoke 执行函数调用的核心逻辑
func runInvoke(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("please provide a name for the function")
	}
	functionName = args[0]

	// 从 yaml 中获取命名空间
	var stackNamespace string
	if services != nil {
		if function, ok := services.Functions[functionName]; ok {
			if len(function.Namespace) > 0 {
				stackNamespace = function.Namespace
			}
		}
	}
	functionNamespace = getNamespace(functionInvokeNamespace, stackNamespace)

	// 校验签名参数必须同时存在
	if missingSignFlag(sigHeader, key) {
		return fmt.Errorf("signing requires both --sign <header-value> and --key <key-value>")
	}

	// 校验 HTTP 方法是否合法
	err := validateHTTPMethod(httpMethod)
	if err != nil {
		return nil
	}

	// 解析请求头
	httpHeader, err := parseHeaders(headers)
	if err != nil {
		return err
	}

	// 解析查询参数
	httpQuery, err := parseQueryValues(query)
	if err != nil {
		return err
	}

	// 设置 Content-Type
	if httpHeader.Get("Content-Type") == "" || cmd.Flag("content-type").Changed {
		httpHeader.Set("Content-Type", contentType)
	}

	// 设置 User-Agent
	httpHeader.Set("User-Agent", fmt.Sprintf("faas-cli/%s (openfaas; %s; %s)", version.BuildVersion(), runtime.GOOS, runtime.GOARCH))

	// 读取标准输入作为函数请求体
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		fmt.Fprintf(os.Stderr, "Reading from STDIN - hit (Control + D) to stop.\n")
	}

	functionInput, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("unable to read standard input: %s", err.Error())
	}

	// 如果需要，生成 HMAC 签名
	if len(sigHeader) > 0 {
		sig := generateSignature(functionInput, key)
		httpHeader.Add(sigHeader, sig)
	}

	// 创建 SDK 客户端
	client, err := GetDefaultSDKClient()
	if err != nil {
		return err
	}

	// 构造请求 URL
	u, _ := url.Parse("/")
	u.RawQuery = httpQuery.Encode()

	// 发送第一次调用请求
	body := bytes.NewReader(functionInput)
	req, err := http.NewRequest(httpMethod, u.String(), body)
	if err != nil {
		return err
	}
	req.Header = httpHeader

	res, err := client.InvokeFunction(functionName, functionNamespace, invokeAsync, authenticate, req)
	if err != nil {
		return fmt.Errorf("failed to invoke function: %s", err)
	}
	if res.Body != nil {
		defer func() {
			_, _ = io.Copy(io.Discard, res.Body) // 清空响应体
			_ = res.Body.Close()
		}()
	}

	// 如果第一次返回 401，则自动重试并启用认证
	if !authenticate && res.StatusCode == http.StatusUnauthorized {
		authenticateHeader := res.Header.Get("WWW-Authenticate")
		realm := getRealm(authenticateHeader)

		// 如果是函数调用认证领域，则自动重试
		if realm == functionInvokeRealm {
			authenticate := true
			body := bytes.NewReader(functionInput)
			req, err := http.NewRequest(httpMethod, u.String(), body)
			if err != nil {
				return err
			}
			req.Header = httpHeader

			res, err = client.InvokeFunction(functionName, functionInvokeNamespace, invokeAsync, authenticate, req)
			if err != nil {
				return fmt.Errorf("failed to invoke function: %s", err)
			}
			if res.Body != nil {
				defer func() {
					_, _ = io.Copy(io.Discard, res.Body)
					_ = res.Body.Close()
				}()
			}
		}
	}

	// 检查响应状态码
	if code := res.StatusCode; code < 200 || code > 299 {
		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("cannot read result from OpenFaaS on URL: %s %s", gateway, err)
		}

		return fmt.Errorf("server returned unexpected status code: %d - %s", res.StatusCode, string(resBody))
	}

	// 异步调用成功提示
	if invokeAsync && res.StatusCode == http.StatusAccepted {
		fmt.Fprintf(os.Stderr, "Function submitted asynchronously.\n")
		return nil
	}

	// 输出响应结果到标准输出
	if _, err := io.Copy(os.Stdout, res.Body); err != nil {
		return fmt.Errorf("cannot read result from OpenFaaS on URL: %s %s", gateway, err)
	}

	return nil
}

// generateSignature 生成 SHA256 HMAC 签名
func generateSignature(message []byte, key string) string {
	hash := hmac.Sign(message, []byte(key), crypto.SHA256.New)
	signature := hex.EncodeToString(hash)

	return fmt.Sprintf(`%s=%s`, "sha256", string(signature[:]))
}

// missingSignFlag 校验签名参数是否完整
func missingSignFlag(header string, key string) bool {
	return (len(header) > 0 && len(key) == 0) || (len(header) == 0 && len(key) > 0)
}

// parseHeaders 解析请求头，支持 Key: Value 格式
func parseHeaders(headers []string) (http.Header, error) {
	httpHeader := http.Header{}
	warningShown := false

	for _, header := range headers {
		// 优先使用标准格式 Key: Value
		parts := strings.SplitN(header, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			if key == "" {
				return httpHeader, fmt.Errorf("the --header or -H flag must take the form of 'Key: Value' (empty key given)")
			}

			value := strings.TrimSpace(parts[1])
			if value == "" {
				return httpHeader, fmt.Errorf("the --header or -H flag must take the form of 'Key: Value' (empty value given)")
			}

			httpHeader.Add(key, value)
			continue
		}

		// 兼容旧版 key=value 格式
		parts = strings.SplitN(header, "=", 2)
		if len(parts) == 2 {
			key := parts[0]
			if key == "" {
				return httpHeader, fmt.Errorf("the --header or -H flag must take the form of 'Key: Value' or 'key=value' (empty key given)")
			}

			value := parts[1]
			if value == "" {
				return httpHeader, fmt.Errorf("the --header or -H flag must take the form of 'Key: Value' or 'key=value' (empty value given)")
			}

			// 只显示一次弃用警告
			if !warningShown {
				fmt.Fprintf(os.Stderr, "Warning: Using deprecated 'key=value' format for headers. Please use 'Key: Value' format instead.\n")
				warningShown = true
			}
			httpHeader.Add(key, value)
			continue
		}

		return httpHeader, fmt.Errorf("the --header or -H flag must take the form of 'Key: Value' or 'key=value'")
	}

	return httpHeader, nil
}

// parseQueryValues 解析 ?key=value 查询参数
func parseQueryValues(query []string) (url.Values, error) {
	v := url.Values{}

	for _, q := range query {
		queryVal := strings.SplitN(q, "=", 2)
		if len(queryVal) != 2 {
			return v, fmt.Errorf("the --query flag must take the form of key=value")
		}

		key, value := queryVal[0], queryVal[1]
		if key == "" {
			return v, fmt.Errorf("the --header or -H flag must take the form of key=value (empty key given)")
		}

		if value == "" {
			return v, fmt.Errorf("the --header or -H flag must take the form of key=value (empty value given)")
		}

		v.Add(key, value)
	}

	return v, nil
}

// validateHTTPMethod 校验 HTTP 方法是否合法
func validateHTTPMethod(httpMethod string) error {
	var allowedMethods = []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
	}
	helpString := strings.Join(allowedMethods, "/")

	if !contains(allowedMethods, httpMethod) {
		return fmt.Errorf("the --method or -m flag must take one of these values (%s)", helpString)
	}
	return nil
}

// getRealm 解析 WWW-Authenticate 头获取 realm 字段（简易解析）
func getRealm(headerVal string) string {
	parts := strings.SplitN(headerVal, " ", 2)

	realm := ""
	if len(parts) > 1 {
		directives := strings.Split(parts[1], ", ")

		for _, part := range directives {
			if strings.HasPrefix(part, "realm=") {
				realm = strings.Trim(part[6:], `"`)
				break
			}
		}
	}

	return realm
}
