#!/bin/sh

# 从已经构建好的 Docker 镜像里，把「Linux/macOS/Windows/ARM」全平台编译好的 faas-cli 全部提取出来
# ，放到本地 bin/ 目录，方便直接分发使用

# 定义默认镜像标签：latest-dev（如果不传参数则用这个）
export eTAG="latest-dev"

# 打印输入的第1个参数（用于调试）
echo $1

# 如果用户传入了版本标签，就替换默认标签
if [ $1 ] ; then
  eTAG=$1
fi

# ==============================
# 脚本说明：
# 从 openfaas/faas-cli:${eTAG} 镜像中
# 提取所有平台的编译好的 CLI 二进制文件
# 注意：必须先运行 ./build.sh 构建镜像
# ==============================

# 1. 创建临时容器，用于拷贝文件
docker create --name faas-cli openfaas/faas-cli:${eTAG} && \

# 2. 创建本地 bin 目录（存放多平台二进制）
mkdir -p ./bin && \

# 3. 从容器中复制 各个平台 的 faas-cli 到本地 bin/
docker cp faas-cli:/home/app/faas-cli ./bin &&                # Linux amd64
docker cp faas-cli:/home/app/faas-cli-darwin ./bin &&          # macOS amd64
docker cp faas-cli:/home/app/faas-cli-darwin-arm64 ./bin &&    # macOS M1/M2/M3
docker cp faas-cli:/home/app/faas-cli-armhf ./bin &&           # ARM 32位
docker cp faas-cli:/home/app/faas-cli-arm64 ./bin &&           # ARM 64位 (ARMv8)
docker cp faas-cli:/home/app/faas-cli.exe ./bin &&             # Windows

# 4. 删除临时容器
docker rm -f faas-cli