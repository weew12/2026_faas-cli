// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/openfaas/go-sdk/stack"

	types "github.com/openfaas/faas-provider/types"
)

var (
	defaultCommandTimeout = 60 * time.Second
)

// FunctionResourceRequest 定义用于设置函数资源的请求体
type FunctionResourceRequest struct {
	Limits   *stack.FunctionResources
	Requests *stack.FunctionResources
}

// DeployFunctionSpec 定义部署函数时使用的规格
type DeployFunctionSpec struct {
	FProcess                string
	FunctionName            string
	Image                   string
	RegistryAuth            string
	Language                string
	Replace                 bool
	EnvVars                 map[string]string
	Network                 string
	Constraints             []string
	Update                  bool
	Secrets                 []string
	Labels                  map[string]string
	Annotations             map[string]string
	FunctionResourceRequest FunctionResourceRequest
	ReadOnlyRootFilesystem  bool
	TLSInsecure             bool
	Token                   string
	Namespace               string
}

func generateFuncStr(spec *DeployFunctionSpec) string {

	if len(spec.Namespace) > 0 {
		return fmt.Sprintf("%s.%s", spec.FunctionName, spec.Namespace)
	}
	return spec.FunctionName
}

// DeployFunction 首先尝试部署函数，如果函数已存在则尝试滚动更新。
// 第二次调用 API 时会抑制警告（如果需要）。
func (c *Client) DeployFunction(context context.Context, spec *DeployFunctionSpec) int {

	rollingUpdateInfo := fmt.Sprintf("Function %s already exists, attempting rolling-update.", spec.FunctionName)
	statusCode, deployOutput := c.deploy(context, spec, spec.Update)

	if spec.Update == true && statusCode == http.StatusNotFound {
		// Re-run the function with update=false

		statusCode, deployOutput = c.deploy(context, spec, false)
	} else if statusCode == http.StatusOK {
		fmt.Println(rollingUpdateInfo)
	}
	fmt.Println()
	fmt.Println(deployOutput)
	return statusCode
}

// deploy 通过 REST 向 OpenFaaS 网关部署函数
func (c *Client) deploy(context context.Context, spec *DeployFunctionSpec, update bool) (int, string) {

	var deployOutput string
	// 需要修改网关以允许 nil/空字符串作为 fprocess，从而避免这种重复代码。
	var fprocessTemplate string
	if len(spec.FProcess) > 0 {
		fprocessTemplate = spec.FProcess
	}

	if spec.Replace {
		c.DeleteFunction(context, spec.FunctionName, spec.Namespace)
	}

	req := types.FunctionDeployment{
		EnvProcess:             fprocessTemplate,
		Image:                  spec.Image,
		Service:                spec.FunctionName,
		EnvVars:                spec.EnvVars,
		Constraints:            spec.Constraints,
		Secrets:                spec.Secrets,
		Labels:                 &spec.Labels,
		Annotations:            &spec.Annotations,
		ReadOnlyRootFilesystem: spec.ReadOnlyRootFilesystem,
		Namespace:              spec.Namespace,
	}

	hasLimits := false
	req.Limits = &types.FunctionResources{}
	if spec.FunctionResourceRequest.Limits != nil && len(spec.FunctionResourceRequest.Limits.Memory) > 0 {
		hasLimits = true
		req.Limits.Memory = spec.FunctionResourceRequest.Limits.Memory
	}
	if spec.FunctionResourceRequest.Limits != nil && len(spec.FunctionResourceRequest.Limits.CPU) > 0 {
		hasLimits = true
		req.Limits.CPU = spec.FunctionResourceRequest.Limits.CPU
	}
	if !hasLimits {
		req.Limits = nil
	}

	hasRequests := false
	req.Requests = &types.FunctionResources{}
	if spec.FunctionResourceRequest.Requests != nil && len(spec.FunctionResourceRequest.Requests.Memory) > 0 {
		hasRequests = true
		req.Requests.Memory = spec.FunctionResourceRequest.Requests.Memory
	}
	if spec.FunctionResourceRequest.Requests != nil && len(spec.FunctionResourceRequest.Requests.CPU) > 0 {
		hasRequests = true
		req.Requests.CPU = spec.FunctionResourceRequest.Requests.CPU
	}

	if !hasRequests {
		req.Requests = nil
	}

	reqBytes, _ := json.Marshal(&req)
	reader := bytes.NewReader(reqBytes)
	var request *http.Request

	method := http.MethodPost
	// "application/json"
	if update {
		method = http.MethodPut
	}

	query := url.Values{}

	var err error
	request, err = c.newRequest(method, "/system/functions", query, reader)

	if err != nil {
		deployOutput += fmt.Sprintln(err)
		return http.StatusInternalServerError, deployOutput
	}

	res, err := c.doRequest(context, request)

	if err != nil {
		deployOutput += fmt.Sprintln("Is OpenFaaS deployed? Do you need to specify the --gateway flag?")
		deployOutput += fmt.Sprintln(err)
		return http.StatusInternalServerError, deployOutput
	}

	if res.Body != nil {
		defer func() {
			_, _ = io.Copy(io.Discard, res.Body) // drain to EOF
			_ = res.Body.Close()
		}()
	}

	switch res.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		deployOutput += fmt.Sprintf("Deployed. %s.\n", res.Status)

		deployedURL := fmt.Sprintf("URL: %s/function/%s", c.GatewayURL.String(), generateFuncStr(spec))
		deployOutput += fmt.Sprintln(deployedURL)
	case http.StatusUnauthorized:
		deployOutput += fmt.Sprintln("unauthorized access, run \"faas-cli login\" to setup authentication for this server")

	default:
		bytesOut, err := io.ReadAll(res.Body)
		if err == nil {
			deployOutput += fmt.Sprintf("Unexpected status: %d, message: %s\n", res.StatusCode, string(bytesOut))
		}
	}

	return res.StatusCode, deployOutput
}
