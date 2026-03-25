// Copyright (c) OpenFaaS Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package test 提供单元测试工具集
// 包含HTTP模拟服务器、标准输出捕获等测试辅助功能
package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// Request 定义HTTP模拟请求的期望参数
type Request struct {
	Method             string      // 期望的请求方法（GET/POST等）
	Uri                string      // 期望的请求URI路径
	ResponseStatusCode int         // 要返回的HTTP状态码
	ResponseBody       interface{} // 要返回的响应体（自动序列化为JSON）
}

// server 模拟HTTP服务器实例
type server struct {
	URL                string           // 测试服务器地址（快捷访问）
	server             *httptest.Server // 底层测试服务器
	requestCounter     int              // 已接收请求计数
	nbExpectedRequests int              // 期望接收的总请求数
	t                  *testing.T       // 测试用例实例
}

// MockHttpServer 创建一个有序响应的HTTP测试服务器
// 按照传入的请求顺序依次验证请求方法、URI，并返回预设响应
func MockHttpServer(t *testing.T, requests []Request) *server {
	s := server{
		requestCounter:     0,
		nbExpectedRequests: len(requests),
		t:                  t,
	}

	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request Request
		request, requests = requests[0], requests[1:]

		// 验证请求方法
		if len(request.Method) > 0 && r.Method != request.Method {
			t.Fatalf(
				"Request n° %d: Expected Method '%s' but got '%s'",
				s.requestCounter+1,
				request.Method,
				r.Method,
			)
		}

		// 验证请求URI
		if len(request.Uri) > 0 && r.RequestURI != request.Uri {
			t.Fatalf(
				"Request n° %d: Expected Uri '%s' but got '%s'",
				s.requestCounter+1,
				request.Uri,
				r.RequestURI,
			)
		}

		w.Header().Add("Content-Type", "application/json")

		// 设置响应状态码，默认200
		if request.ResponseStatusCode > 0 {
			w.WriteHeader(request.ResponseStatusCode)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		// 写入响应体，支持字符串或自动JSON序列化
		if request.ResponseBody != nil {
			strBody, ok := request.ResponseBody.(string)
			if !ok {
				jsonBody, err := json.Marshal(request.ResponseBody)
				if err != nil {
					t.Fatal(err)
				}
				w.Write(jsonBody)
			} else {
				w.Write([]byte(strBody))
			}
		}

		s.requestCounter++
	}))

	s.URL = s.server.URL

	return &s
}

// MockHttpServerStatus 创建仅返回状态码的HTTP测试服务器
// 按传入的状态码序列依次返回空响应
func MockHttpServerStatus(t *testing.T, statusCode ...int) *server {
	var requests []Request
	for _, s := range statusCode {
		requests = append(requests, Request{
			ResponseStatusCode: s,
		})
	}

	return MockHttpServer(t, requests)
}

// Close 关闭测试服务器
// 关闭前自动校验请求数量是否符合预期
func (s *server) Close() {
	s.server.Close()
	s.assertNbRequests()
}

// assertNbRequests 校验实际接收请求数与预期是否一致
// 不一致则触发测试失败
func (s *server) assertNbRequests() {
	if s.nbExpectedRequests != s.requestCounter {
		s.t.Fatalf(
			"Expected %d requests but received %d",
			s.nbExpectedRequests,
			s.requestCounter,
		)
	}
}

// CaptureStdout 捕获函数执行时的标准输出并返回字符串
// 用于测试打印类输出逻辑
func CaptureStdout(f func()) string {
	// 保存原始标准输出
	originalStdout := os.Stdout
	// 创建管道捕获输出
	readPipe, writePipe, _ := os.Pipe()
	defer readPipe.Close()

	os.Stdout = writePipe
	// 执行目标函数
	f()

	// 恢复标准输出
	writePipe.Close()
	os.Stdout = originalStdout

	// 读取捕获内容
	var buffer bytes.Buffer
	io.Copy(&buffer, readPipe)

	return buffer.String()
}
