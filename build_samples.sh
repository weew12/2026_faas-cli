#!/bin/sh

# 构建 OpenFaaS 函数 + 查看镜像

# 1. 调用 faas-cli 执行【构建函数镜像】
# 等价于：faas-cli build -f stack.yml
# 会根据当前目录的 yaml 文件构建 Docker 镜像
./bin/faas-cli build # --squash=true

# 2. 列出本地所有 Docker 镜像，只显示前 4 行（方便查看刚构建的函数镜像）
docker images |head -n 4