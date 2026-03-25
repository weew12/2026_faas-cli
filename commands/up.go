// Copyright (c) OpenFaaS Author(s) 2018. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 up 命令，一键完成函数的构建、推送与部署
package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// 全局命令行标志变量
var (
	skipPush   bool // 是否跳过推送镜像到仓库
	skipDeploy bool // 是否跳过部署函数
	usePublish bool // 是否使用 publish 命令替代 build+push
	watch      bool // 是否监听文件变化自动重新部署
)

// init 初始化 up 命令，注册命令行参数并继承 build/push/deploy 命令的参数
func init() {
	// 创建 up 命令独立参数集合
	upFlagset := pflag.NewFlagSet("up", pflag.ExitOnError)
	upFlagset.BoolVar(&usePublish, "publish", false, "Use faas-cli publish instead of faas-cli build followed by faas-cli push")
	upFlagset.StringVar(&platforms, "platforms", "linux/amd64", "Publish for these platforms, when used with --publish")

	upFlagset.BoolVar(&skipPush, "skip-push", false, "Skip pushing function to remote registry")
	upFlagset.BoolVar(&skipDeploy, "skip-deploy", false, "Skip function deployment")
	upFlagset.StringVar(&remoteBuilder, "remote-builder", "", "URL to the builder")
	upFlagset.StringVar(&payloadSecretPath, "payload-secret", "", "Path to the payload secret file")
	upFlagset.StringVar(&builderPublicKeyPath, "builder-public-key", "", "Builder public key as a literal value, or a path to a file containing raw base64 or the JSON response from /publickey")

	upFlagset.BoolVar(&watch, "watch", false, "Watch for changes in files and re-deploy")
	// 添加独立参数
	upCmd.Flags().AddFlagSet(upFlagset)

	// 继承 build 命令的所有参数
	build, _, _ := faasCmd.Find([]string{"build"})
	upCmd.Flags().AddFlagSet(build.Flags())

	// 继承 push 命令的所有参数
	push, _, _ := faasCmd.Find([]string{"push"})
	upCmd.Flags().AddFlagSet(push.Flags())

	// 继承 deploy 命令的所有参数
	deploy, _, _ := faasCmd.Find([]string{"deploy"})
	upCmd.Flags().AddFlagSet(deploy.Flags())

	// 将 up 命令注册到根命令
	faasCmd.AddCommand(upCmd)
}

// upCmd 是 build、push、deploy 命令的封装命令
var upCmd = &cobra.Command{
	Use:   `up -f [YAML_FILE] [--skip-push] [--skip-deploy] [flags from build, push, deploy]`,
	Short: "Builds, pushes and deploys OpenFaaS function containers",
	Long: `Build, Push, and Deploy OpenFaaS function containers either via the
supplied YAML config using the "--yaml" flag (which may contain multiple function
definitions), or directly via flags.

The push step may be skipped by setting the --skip-push flag
and the deploy step with --skip-deploy.

Note: All flags from the build, push and deploy flags are valid and can be combined,
see the --help text for those commands for details.`,
	Example: `  # Deploy everything
  faas-cli up

  # Deploy a named function
  faas-cli up --filter echo

  # Deploy but skip the push step
  faas-cli up --skip-push

  # Build but skip pushing and use a build-arg
  faas-cli up --skip-push \
  	--build-arg GO111MODULE=on

  # Publish with a remote builder and auto-discover /public-key
  faas-cli up --publish \
    --remote-builder http://127.0.0.1:8081 \
    --payload-secret /var/openfaas/secrets/payload-secret \
    -f stack.yml
	`,
	PreRunE: preRunUp,
	RunE:    upHandler,
}

// preRunUp up 命令前置检查
// 执行 build 和 deploy 命令的前置校验逻辑
func preRunUp(cmd *cobra.Command, args []string) error {
	if err := preRunBuild(cmd, args); err != nil {
		return err
	}
	if err := preRunDeploy(cmd, args); err != nil {
		return err
	}
	return nil
}

// upHandler up 命令入口处理函数
// 根据 watch 标志决定启动热重载监听或直接执行一次构建部署
func upHandler(cmd *cobra.Command, args []string) error {
	if watch {
		// 启动文件监听，文件变化时自动重新构建部署
		return watchLoop(cmd, args, func(cmd *cobra.Command, args []string, ctx context.Context) error {
			if err := upRunner(cmd, args); err != nil {
				return err
			}
			fmt.Println("[Watch] Change a file to trigger a rebuild...")
			return nil
		})
	}

	// 直接执行构建部署流程
	return upRunner(cmd, args)
}

// upRunner 执行构建、推送、部署的核心流程
// 根据参数选择使用 publish 或 build+push 流程
func upRunner(cmd *cobra.Command, args []string) error {
	if usePublish {
		// 使用 publish 命令（多平台构建+推送）
		if err := runPublish(cmd, args); err != nil {
			return err
		}
	} else {
		// 参数校验：--platforms 只能与 --publish 一起使用
		if len(platforms) > 0 && cmd.Flags().Changed("platforms") {
			return fmt.Errorf("--platforms can only be used with the --publish flag")
		}

		// 执行构建
		if err := runBuild(cmd, args); err != nil {
			return err
		}

		// 未跳过推送且无远程构建器时执行推送
		if !skipPush && remoteBuilder == "" {
			if err := runPush(cmd, args); err != nil {
				return err
			}
		}
	}

	// 未跳过部署则执行部署
	if !skipDeploy {
		if err := runDeploy(cmd, args); err != nil {
			return err
		}
	}

	return nil
}

// ignorePatterns 读取 .gitignore 文件并解析为 gitignore 匹配规则
// 用于文件监听时过滤不需要监控的文件
// 返回：gitignore 规则列表、读取错误
func ignorePatterns() ([]gitignore.Pattern, error) {
	gitignorePath := ".gitignore"

	file, err := os.Open(gitignorePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// 默认忽略 .git 目录
	patterns := []gitignore.Pattern{gitignore.ParsePattern(".git", nil)}

	// 逐行解析 .gitignore
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, gitignore.ParsePattern(line, nil))
	}

	// 检查扫描过程是否出错
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return patterns, nil
}
