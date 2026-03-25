# ==============================
# 阶段 1：许可证检查镜像
# 作用：检查所有 Go 文件是否有合法许可证头
# ==============================
FROM ghcr.io/openfaas/license-check:0.4.2 as license-check

# ==============================
# 阶段 2：编译构建器（核心编译阶段）
# ==============================
FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.25 as builder

# 由 Docker Buildx 自动传入：目标平台/系统/架构
ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

# 由 Makefile 传入：版本、Git 提交号
ARG GIT_COMMIT
ARG VERSION

# Go 环境配置
ENV GO111MODULE=on
ENV GOFLAGS=-mod=vendor  # 使用 vendor 目录
ENV CGO_ENABLED=0       # 静态编译，不依赖C库

WORKDIR /usr/bin/

# 把许可证检查工具复制到 builder 中
COPY --from=license-check /license-check /usr/bin/

# 复制项目源码
WORKDIR /go/src/github.com/openfaas/faas-cli
COPY . .

# ==============================
# 代码风格检查：gofmt
# 不格式化直接报错，强制代码规范
# ==============================
RUN test -z "$(gofmt -l $(find . -type f -name '*.go' -not -path "./vendor/*"))" || { echo "Run \"gofmt -s -w\" on your Golang code"; exit 1; }

# ==============================
# 许可证检查
# 检查所有文件是否包含正确的版权头
# ==============================
RUN /usr/bin/license-check -path ./ --verbose=false "Alex Ellis" "OpenFaaS Author(s)" "OpenFaaS Ltd"

# ==============================
# 运行单元测试（排除不需要的目录）
# ==============================
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go test $(go list ./... | grep -v /vendor/ | grep -v /template/|grep -v /build/|grep -v /sample/) -cover

# ==============================
# 【核心】跨平台编译 faas-cli
# -s -w：减小体积
# -X：注入版本、commit、平台信息
# ==============================
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 \
    go build --ldflags "-s -w \
    -X github.com/openfaas/faas-cli/version.GitCommit=${GIT_COMMIT} \
    -X github.com/openfaas/faas-cli/version.Version=${VERSION} \
    -X github.com/openfaas/faas-cli/commands.Platform=${TARGETARCH}" \
   -o faas-cli

# ==============================
# 阶段 3：root 权限镜像（用于特殊场景）
# ==============================
FROM --platform=${TARGETPLATFORM:-linux/amd64} alpine:3.22.1 as root

ARG REPO_URL
LABEL org.opencontainers.image.source $REPO_URL

# 安装证书、git（用于拉模板）
RUN apk --no-cache add ca-certificates git

WORKDIR /home/app

# 从 builder 阶段复制编译好的 CLI
COPY --from=builder /go/src/github.com/openfaas/faas-cli/faas-cli /usr/bin/

ENV PATH=$PATH:/usr/bin/

# 以 root 身份运行
ENTRYPOINT [ "faas-cli" ]

# ==============================
# 阶段 4：普通用户权限镜像（官方推荐、安全、最终发布版）
# ==============================
FROM --platform=${TARGETPLATFORM:-linux/amd64} alpine:3.21.0 as release

ARG REPO_URL
LABEL org.opencontainers.image.source $REPO_URL

RUN apk --no-cache add ca-certificates git

# 创建普通用户 app，不使用 root，更安全
RUN addgroup -S app \
    && adduser -S -g app app \
    && apk add --no-cache ca-certificates

WORKDIR /home/app

# 复制二进制
COPY --from=builder /go/src/github.com/openfaas/faas-cli/faas-cli /usr/bin/
RUN chown -R app:app ./

# 切换为普通用户
USER app

ENV PATH=$PATH:/usr/bin/

ENTRYPOINT ["faas-cli"]