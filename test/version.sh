#!/bin/bash

# 验证 faas-cli 编译时注入的 Git 版本号是否正确。

# 接收第1个参数：faas-cli 二进制文件路径
FAAS_BINARY=$1

# 获取本地当前最新的 Git 提交哈希值
GIT_COMMIT=$(git rev-list -1 HEAD)

# 打印测试提示
echo "TEST: Version command prints GitCommit"

# 执行 faas-cli version
# 1. 输出到屏幕（stderr）
# 2. 同时用 grep 检查是否包含正确的 Git commit
${FAAS_BINARY} version | tee /dev/stderr | grep "commit:  ${GIT_COMMIT}"

# 判断上一条命令（grep）是否执行成功
if [ $? -eq 0 ]; then
    # 成功：输出 OK
    echo OK
else
    # 失败：输出 FAIL 并退出错误码 1
    echo FAIL
    exit 1
fi