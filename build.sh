#!/bin/bash

# 构建 2 种 OpenFaaS CLI 镜像，并把编译好的二进制文件提取到本地。
# 构建 2 个镜像
# openfaas/faas-cli:xxx
# 普通用户权限，安全，推荐使用
# openfaas/faas-cli:xxx-root
# root 权限，用于特殊场景

# 定义默认镜像标签：latest-dev（如果没有传入参数则用这个）
export eTAG="latest-dev"

# 打印第 1 个输入参数（用于调试）
echo $1

# 如果用户传入了参数（比如 ./build.sh 0.1.2），就覆盖默认标签
if [ $1 ] ; then
  eTAG=$1
fi

# 提示：开始构建普通用户镜像
echo Building openfaas/faas-cli:$eTAG

# 构建【普通用户权限】镜像（target: release）
# 传入代理，使用 Dockerfile 中的 release 阶段
docker build \
  --build-arg http_proxy=$http_proxy \
  --build-arg https_proxy=$https_proxy \
  --target release \
  -t openfaas/faas-cli:$eTAG .

# 提示：开始构建 root 镜像
echo Building openfaas/faas-cli:$eTAG-root

# 构建【root 权限】镜像（target: root）
docker build \
  --build-arg http_proxy=$http_proxy \
  --build-arg https_proxy=$https_proxy \
  --target root \
  -t openfaas/faas-cli:$eTAG-root .

# ==================================================
# 如果上一步构建成功（返回码 0）
# 就从镜像里把编译好的 faas-cli 二进制文件复制出来
# ==================================================
if [ $? == 0 ] ; then

  # 创建临时容器
  docker create --name faas-cli openfaas/faas-cli:$eTAG && \
  
  # 从容器内 /usr/bin/faas-cli 复制到宿主机当前目录
  docker cp faas-cli:/usr/bin/faas-cli . && \
  
  # 删除临时容器
  docker rm -f faas-cli

else
  # 构建失败 → 退出并返回错误码
  exit 1
fi