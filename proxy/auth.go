package proxy

import (
	"net/http"

	"github.com/openfaas/faas-cli/config"
)

// CLIAuth CLI 认证结构体
type CLIAuth struct {
	Username string
	Password string
	Token    string
}

// BasicAuth 基础认证类型
type BasicAuth struct {
	username string
	password string
}

// Set 将基础认证信息设置到 HTTP 请求头
func (auth *BasicAuth) Set(req *http.Request) error {
	req.SetBasicAuth(auth.username, auth.password)
	return nil
}

// BearerToken Bearer Token 认证类型
type BearerToken struct {
	token string
}

// Set 将 Token 认证信息设置到 HTTP 请求头
func (c *BearerToken) Set(req *http.Request) error {
	req.Header.Set("Authorization", "Bearer "+c.token)
	return nil
}

// NewCLIAuth 创建 CLI 认证对象
func NewCLIAuth(token string, gateway string) (ClientAuth, error) {
	authConfig, _ := config.LookupAuthConfig(gateway)

	var (
		username    string
		password    string
		bearerToken string
		err         error
	)

	if authConfig.Auth == config.BasicAuthType {
		username, password, err = config.DecodeAuth(authConfig.Token)
		if err != nil {
			return nil, err
		}

		return &BasicAuth{
			username: username,
			password: password,
		}, nil

	}

	if len(token) > 0 {
		bearerToken = token
	} else {
		bearerToken = authConfig.Token
	}

	return &BearerToken{
		token: bearerToken,
	}, nil
}
