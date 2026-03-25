package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	gopath "path"
	"strings"
	"time"

	"github.com/openfaas/faas-cli/version"
)

// Client 用于执行所有操作的 API 客户端
type Client struct {
	httpClient *http.Client
	// ClientAuth 实现了 ClientAuth 接口的客户端认证类型
	ClientAuth ClientAuth
	// GatewayURL OpenFaaS 网关的基础 URL
	GatewayURL *url.URL
	// UserAgent 客户端使用的用户代理
	UserAgent string
}

// ClientAuth 客户端认证接口。
// 若要为客户端添加认证方式，请实现此接口
type ClientAuth interface {
	Set(req *http.Request) error
}

// NewClient 初始化一个新的 API 客户端
func NewClient(auth ClientAuth, gatewayURL string, transport http.RoundTripper, timeout *time.Duration) (*Client, error) {
	gatewayURL = strings.TrimRight(gatewayURL, "/")
	baseURL, err := url.Parse(gatewayURL)
	if err != nil {
		return nil, fmt.Errorf("invalid gateway URL: %s", gatewayURL)
	}

	client := &http.Client{}
	if timeout != nil {
		client.Timeout = *timeout
	}

	if transport != nil {
		client.Transport = transport
	}

	return &Client{
		ClientAuth: auth,
		httpClient: client,
		GatewayURL: baseURL,
		UserAgent:  fmt.Sprintf("faas-cli/%s", version.BuildVersion()),
	}, nil
}

// newRequest 创建一个带有认证的新 HTTP 请求
func (c *Client) newRequest(method, path string, query url.Values, body io.Reader) (*http.Request, error) {

	// 深度拷贝网关地址，然后将路径与参数追加到副本上
	// 最大程度保留原始网关 URL
	endpoint, err := url.Parse(c.GatewayURL.String())
	if err != nil {
		return nil, err
	}

	endpoint.Path = gopath.Join(endpoint.Path, path)
	endpoint.RawQuery = query.Encode()

	bodyDebug := ""
	if os.Getenv("FAAS_DEBUG") == "1" {

		if body != nil {
			r := io.NopCloser(body)
			buf := new(strings.Builder)
			_, err := io.Copy(buf, r)
			if err != nil {
				return nil, err
			}
			bodyDebug = buf.String()
			body = io.NopCloser(strings.NewReader(buf.String()))
		}
	}

	req, err := http.NewRequest(method, endpoint.String(), body)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	c.ClientAuth.Set(req)

	if os.Getenv("FAAS_DEBUG") == "1" {
		fmt.Printf("%s %s\n", req.Method, req.URL.String())
		for k, v := range req.Header {
			if k == "Authorization" {
				auth := "[REDACTED]"
				if len(v) == 0 {
					auth = "[NOT_SET]"
				} else {
					l, _, ok := strings.Cut(v[0], " ")
					if ok && (l == "Basic" || l == "Bearer") {
						auth = l + " REDACTED"
					}
				}
				fmt.Printf("%s: %s\n", k, auth)

			} else {
				fmt.Printf("%s: %s\n", k, v)
			}
		}

		if len(bodyDebug) > 0 {
			fmt.Printf("%s\n", bodyDebug)
		}
	}

	return req, err
}

// doRequest 执行带有上下文的 HTTP 请求
func (c *Client) doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)

	if val, ok := os.LookupEnv("OPENFAAS_DUMP_HTTP"); ok && val == "true" {
		dump, err := httputil.DumpRequest(req, true)
		if err != nil {
			return nil, err
		}
		fmt.Println(string(dump))
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	return res, err
}

func addQueryParams(u string, params map[string]string) (string, error) {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return u, err
	}

	qs := parsedURL.Query()
	for key, value := range params {
		qs.Add(key, value)
	}
	parsedURL.RawQuery = qs.Encode()
	return parsedURL.String(), nil
}

// AddCheckRedirect 为客户端添加重定向检查
func (c *Client) AddCheckRedirect(checkRedirect func(*http.Request, []*http.Request) error) {
	c.httpClient.CheckRedirect = checkRedirect
}
