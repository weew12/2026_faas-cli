#!/bin/bash

# 这是 OpenFaaS Python3 模板的全自动端到端测试脚本
# 作用：
# 拉取模板
# 自动识别系统 / 架构使用对应 CLI
# 创建函数
# 构建镜像
# 启动容器
# 测试输出是否正确
# 自动清理容器 / 镜像 / 文件

# 脚本执行选项（非常重要，生产标准配置）
set -e            # 只要有命令报错，立刻停止整个脚本（防止错误扩散）
set -x            # 执行前打印每条命令（调试用）
set -o pipefail   # 管道中任意一步报错，整个命令都算失败

# 定义 CLI 路径（默认使用本地编译好的 faas-cli）
cli="./bin/faas-cli"

# 要测试的函数模板名称：Python3 HTTP 模板
TEMPLATE_NAME="python3-http"

# ======================================
# 函数1：根据系统/架构自动选择对应 faas-cli 二进制
# ======================================
get_package() {
    uname=$(uname)          # 获取系统类型：Linux/Darwin
    arch=$(uname -m)        # 获取架构：x86_64/arm64/armv7l
    echo "Getting faas-cli package for $uname..."
    echo "Having architecture $arch..."

    # 根据系统类型选择 CLI
    case $uname in
    "Darwin")  # macOS
        cli="./faas-cli-darwin"
        case $arch in  # M1/M2/M3
        "arm64")
        cli="./faas-cli-darwin-arm64"
        ;;
        esac
    ;;
    "Linux")   # Linux
        case $arch in
        "armv6l" | "armv7l")  # ARM 32位
        cli="./faas-cli-armhf"
        ;;
        esac
    ;;
    esac

    echo "Using package $cli"
    echo "Using template $TEMPLATE_NAME"
}

# ======================================
# 函数2：创建 + 构建 OpenFaaS 函数
# ======================================
build_faas_function() {
    function_name=$1  # 接收函数名参数

    # 用 faas-cli 创建新函数（使用指定模板）
    eval $cli new $function_name --lang $TEMPLATE_NAME

    # 覆盖 handler.py，写入标准测试 handler
cat << EOF > $function_name/handler.py
def handle(event, context):
    return {
        "statusCode": 200,
        "body": {"message": "Hello from OpenFaaS!"},
        "headers": {
            "Content-Type": "application/json"
        }
    }
EOF

    # 构建函数 Docker 镜像
    eval $cli build -f stack.yaml
}

# ======================================
# 函数3：等待函数容器启动成功
# ======================================
wait_for_function_up() {
    function_name=$1
    port=$2
    timeout=$3

    function_up=false
    for (( i=1; i<=$timeout; i++ ))
    do
        echo "Checking if 127.0.0.1:$port is up.. ${i}/$timeout"
        # 调用健康检查接口
        status_code=$(curl 127.0.0.1:$port/_/health -o /dev/null -sw '%{http_code}')
        if [ $status_code == 200 ] ; then
            echo "Function $function_name is up on 127.0.0.1:$port"
            function_up=true
            break
        else
            echo "Waiting for function $function_name to run on 127.0.0.1:$port"
            sleep 1
        fi
    done

    # 超时失败 → 清理环境并退出
    if [ "$function_up" != true ] ; then
        echo "Failed to reach function on 127.0.0.1:$port.. Service timeout"

        echo "Removing container..."
        docker rm -f $id

        echo "Removing function image $function_name:latest"
        docker rmi -f $function_name:latest

        exit 1
    fi
}

# ======================================
# 函数4：启动容器 + 测试函数输出 + 清理环境
# ======================================
run_and_test() {
    function_name=$1
    port=$2
    timeout=$3

    # 检查端口是否被占用
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null ; then
        echo "Port $port is already allocated.. Cannot use this port for testing.
        Exiting test..."

        echo "Removing function image $function_name:latest"
        docker rmi -f $function_name:latest
        exit 1
    fi

    # 启动函数 Docker 容器
    id=$(docker run --env fprocess="python3 index.py" --name $function_name -p $port:8080 -d $function_name:latest)

    # 等待服务启动
    wait_for_function_up $function_name $port $timeout

    # 请求函数，保存输出到 got.txt
    curl -s 127.0.0.1:$port > got.txt

    # 写入期望输出到 want.txt
cat << EOF > want.txt
Function output from integration testing: Hello World!
EOF

    # 对比输出是否一致
    if cmp got.txt want.txt ; then
        echo "SUCCESS testing function $function_name"
    else
        echo "FAILED testing function $function_name"
    fi

    # ====================== 清理 ======================
    echo "Removing container..."
    docker rm -f $id

    echo "Removing function image $function_name:latest"
    docker rmi -f $function_name:latest

    echo "Removing created files..."
    rm -rf got.txt want.txt $function_name*
}

# ======================================
# 函数5：拉取官方模板
# ======================================
get_templates() {
    echo "Getting templates..."
    eval $cli template store pull $TEMPLATE_NAME
}

# ====================== 脚本执行主流程 ======================
get_templates       # 拉取模板
get_package         # 识别系统，选择CLI
build_faas_function $FUNCTION  # 创建并构建函数
run_and_test $FUNCTION $PORT $FUNCTION_UP_TIMEOUT  # 运行+测试+清理