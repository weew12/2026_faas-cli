# OpenFaaS CLI 的自动化构建工具

# 查找所有 .go 文件，排除 vendor 目录（Go 依赖目录）
GO_FILES?=$$(find . -name '*.go' |grep -v vendor)

# 默认镜像标签 latest
TAG?=latest

# 获取当前 Git 提交哈希
.GIT_COMMIT=$(shell git rev-parse HEAD)

# 获取 Git 版本标签（没有标签则使用 commit）
.GIT_VERSION=$(shell git describe --tags 2>/dev/null || echo "$(.GIT_COMMIT)")

# 检查是否有未提交的文件变更
.GIT_UNTRACKEDCHANGES := $(shell git status --porcelain --untracked-files=no)

# 如果有未提交变更，commit 标记为 dirty（表示代码不干净）
ifneq ($(.GIT_UNTRACKEDCHANGES),)
	.GIT_COMMIT := $(.GIT_COMMIT)-dirty
endif

# 使用 vendor 目录进行 Go 构建
export GOFLAGS=-mod=vendor

# ------------------------------
# 伪目标（不是真实文件，只是命令）
# ------------------------------

# 构建主程序 → 执行 build.sh
.PHONY: build
build:
	./build.sh

# 构建发布版 → 提取二进制
.PHONY: build_redist
build_redist:
	./extract_binaries.sh

# 构建示例函数
.PHONY: build_samples
build_samples:
	./build_samples.sh

# 本地代码格式化（gofmt）
.PHONY: local-fmt
local-fmt:
	gofmt -l -d $(GO_FILES)

# 自动导入缺失包、格式化代码
.PHONY: local-goimports
local-goimports:
	goimports -w $(GO_FILES)

# 本地安装 CLI（带版本信息）
.PHONY: local-install
local-install:
	CGO_ENABLED=0 go install --ldflags "-s -w \
	   -X github.com/openfaas/faas-cli/version.GitCommit=${.GIT_COMMIT} \
	   -X github.com/openfaas/faas-cli/version.Version=${.GIT_VERSION}"

# 跨平台编译（Linux/macOS/Windows amd64/arm64）
.PHONY: dist
dist:
	# Linux amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build --ldflags "-s -w \
	   -X github.com/openfaas/faas-cli/version.GitCommit=${.GIT_COMMIT} \
	   -X github.com/openfaas/faas-cli/version.Version=${.GIT_VERSION}" \
	    -o ./bin/faas-cli

	# macOS amd64
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build --ldflags "-s -w \
	   -X github.com/openfaas/faas-cli/version.GitCommit=${.GIT_COMMIT} \
	   -X github.com/openfaas/faas-cli/version.Version=${.GIT_VERSION}" \
	    -o ./bin/faas-cli-darwin

	# macOS arm64 (M1/M2/M3...)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build --ldflags "-s -w \
	   -X github.com/openfaas/faas-cli/version.GitCommit=${.GIT_COMMIT} \
	   -X github.com/openfaas/faas-cli/version.Version=${.GIT_VERSION}" \
	    -o ./bin/faas-cli-darwin-arm64

	# Windows amd64
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build --ldflags "-s -w \
	   -X github.com/openfaas/faas-cli/version.GitCommit=${.GIT_COMMIT} \
	   -X github.com/openfaas/faas-cli/version.Version=${.GIT_VERSION}" \
	    -o ./bin/faas-cli.exe

# 单元测试（排除 vendor、template、build 目录）
.PHONY: test-unit
test-unit:
	go test $(shell go list ./... | grep -v /vendor/ | grep -v /template/ | grep -v build) -cover

# ARMHF 架构 Docker 镜像推送
.PHONY: ci-armhf-push
ci-armhf-push:
	(docker push openfaas/faas-cli:$(TAG)-armhf && docker push openfaas/faas-cli:$(TAG)-root-armhf)

# ARMHF 架构 Docker 镜像构建
.PHONY: ci-armhf-build
ci-armhf-build:
	(./build.sh $(TAG)-armhf)

# ARM64 架构 Docker 镜像推送
.PHONY: ci-arm64-push
ci-arm64-push:
	(docker push openfaas/faas-cli:$(TAG)-arm64 && docker push openfaas/faas-cli:$(TAG)-root-arm64)

# ARM64 架构 Docker 镜像构建
.PHONY: ci-arm64-build
ci-arm64-build:
	(./build.sh $(TAG)-arm64)

# 模板集成测试（用于测试函数模板是否正常）
PORT?=38080
FUNCTION?=templating-test-func
FUNCTION_UP_TIMEOUT?=30
# 导出所有变量给脚本使用
.EXPORT_ALL_VARIABLES:
test-templating:
	./build_integration_test.sh