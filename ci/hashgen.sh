#!/bin/sh

# 给每一个编译好的 faas-cli 二进制文件，生成对应的校验文件，用于验证文件是否被篡改、损坏

# 循环遍历当前目录下 所有以 faas-cli 开头的文件
# 例如：faas-cli、faas-cli-darwin、faas-cli-arm64、faas-cli.exe 等
for f in faas-cli*; do
  # 对每个文件计算 SHA256 哈希值
  # 并把结果写入 【文件名.sha256】 文件中
  shasum -a 256 $f > $f.sha256;
done