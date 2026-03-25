// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 publish 命令，用于**构建并推送多架构函数镜像**，基于 Docker buildx
package commands

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	v2execute "github.com/alexellis/go-execute/v2"
	"github.com/morikuni/aec"
	"github.com/openfaas/faas-cli/util"

	"github.com/openfaas/faas-cli/builder"
	"github.com/openfaas/go-sdk/stack"
	"github.com/spf13/cobra"
)

// 全局命令行参数
var (
	platforms            string   // 多架构平台列表，例如 linux/amd64,linux/arm64
	extraTags            []string // 额外的镜像标签
	resetQemu            bool     // 重置 QEMU 以支持跨架构构建
	mountSSH             bool     // 挂载 SSH 密钥（用于私有仓库拉取）
	remoteBuilder        string   // 远程构建器地址
	payloadSecretPath    string   // 负载密钥路径
	builderPublicKeyPath string   // 构建器公钥路径
	builderKeyID         string   // 构建器密钥ID
)

// init 初始化 publish 命令的所有参数并注册到主命令
func init() {
	// 通用构建参数（与 build 命令共用）
	publishCmd.Flags().StringVar(&image, "image", "", "Docker image name to build")
	publishCmd.Flags().StringVar(&handler, "handler", "", "Directory with handler for function, e.g. handler.js")
	publishCmd.Flags().StringVar(&functionName, "name", "", "Name of the deployed function")
	publishCmd.Flags().StringVar(&language, "lang", "", "Programming language template")

	// 专属构建参数
	publishCmd.Flags().BoolVar(&nocache, "no-cache", false, "Do not use Docker's build cache")
	publishCmd.Flags().BoolVar(&squash, "squash", false, `Use Docker's squash flag for smaller images [experimental] `)
	publishCmd.Flags().IntVar(&parallel, "parallel", 1, "Build in parallel to depth specified.")
	publishCmd.Flags().BoolVar(&shrinkwrap, "shrinkwrap", false, "Just write files to ./build/ folder for shrink-wrapping")
	publishCmd.Flags().StringArrayVarP(&buildArgs, "build-arg", "b", []string{}, "Add a build-arg for Docker (KEY=VALUE)")
	publishCmd.Flags().StringArrayVarP(&buildOptions, "build-option", "o", []string{}, "Set a build option, e.g. dev")
	publishCmd.Flags().Var(&tagFormat, "tag", "Override latest tag on function Docker image, accepts 'latest', 'sha', 'branch', or 'describe'")
	publishCmd.Flags().StringArrayVar(&buildLabels, "build-label", []string{}, "Add a label for Docker image (LABEL=VALUE)")
	publishCmd.Flags().StringArrayVar(&copyExtra, "copy-extra", []string{}, "Extra paths that will be copied into the function build context")
	publishCmd.Flags().BoolVar(&envsubst, "envsubst", true, "Substitute environment variables in stack.yaml file")
	publishCmd.Flags().BoolVar(&quietBuild, "quiet", false, "Perform a quiet build, without showing output from Docker")
	publishCmd.Flags().BoolVar(&disableStackPull, "disable-stack-pull", false, "Disables the template configuration in the stack.yaml")

	// 多架构与远程构建专属参数
	publishCmd.Flags().StringVar(&platforms, "platforms", "linux/amd64", "A set of platforms to publish")
	publishCmd.Flags().StringArrayVar(&extraTags, "extra-tag", []string{}, "Additional extra image tag")
	publishCmd.Flags().BoolVar(&resetQemu, "reset-qemu", false, "Runs \"docker run multiarch/qemu-user-static --reset -p yes\" to enable multi-arch builds. Compatible with AMD64 machines only.")
	publishCmd.Flags().StringVar(&remoteBuilder, "remote-builder", "", "URL to the builder")
	publishCmd.Flags().StringVar(&payloadSecretPath, "payload-secret", "", "Path to the payload secret file")
	publishCmd.Flags().StringVar(&builderPublicKeyPath, "builder-public-key", "", "Builder public key as a literal value, or a path to a file containing raw base64 or the JSON response from /publickey")
	publishCmd.Flags().BoolVar(&forcePull, "pull", false, "Force a re-pull of base images in template during build, useful for publishing images")

	publishCmd.Flags().BoolVar(&pullDebug, "debug", false, "Enable debug output when pulling templates")
	publishCmd.Flags().BoolVar(&overwrite, "overwrite", true, "Overwrite existing templates from the template repository")

	// Bash 自动补全配置
	_ = publishCmd.Flags().SetAnnotation("handler", cobra.BashCompSubdirsInDir, []string{})

	faasCmd.AddCommand(publishCmd)
}

// publishCmd 构建并推送多架构 OpenFaaS 容器镜像
// 基于 Docker buildx，用于生产环境发布，本地不会保留镜像
var publishCmd = &cobra.Command{
	Use: `publish -f YAML_FILE [--no-cache] [--squash]
  faas-cli publish --image IMAGE_NAME
                   --handler HANDLER_DIR
                   --name FUNCTION_NAME
                   [--lang LANG]
                   [--no-cache] [--squash]
                   [--regex "REGEX"]
                   [--filter "WILDCARD"]
                   [--parallel PARALLEL_DEPTH]
                   [--build-arg KEY=VALUE]
                   [--build-option VALUE]
                   [--copy-extra PATH]
                   [--tag <sha|branch|describe>]
                   [--platforms linux/amd64,linux/arm64]
                   [--reset-qemu]
                   [--remote-builder http://127.0.0.1:8081]`,
	Short: "Builds and pushes multi-arch OpenFaaS container images",
	Long: `Builds and pushes multi-arch OpenFaaS container images using Docker buildx.
Most users will want faas-cli build or faas-cli up for development and testing.
This command is designed to make releasing and publishing multi-arch container 
images easier.

A stack.yaml file is required, and any images that are built will not be 
available in the local Docker library. This is due to technical constraints in 
Docker and buildx. You must use a multi-arch template to use this command with 
correctly configured TARGETPLATFORM and BUILDPLATFORM arguments.

See also: faas-cli build`,
	Example: `  faas-cli publish --platforms linux/amd64,linux/arm64
  faas-cli publish --platforms linux/arm64 --filter webhook-arm
  faas-cli publish -f custom.yml --no-cache --build-arg NPM_VERSION=0.2.2
  faas-cli publish --build-option dev
  faas-cli publish --tag sha
  faas-cli publish --reset-qemu
  faas-cli publish --remote-builder http://127.0.0.1:8081 --payload-secret /var/openfaas/secrets/payload-secret -f stack.yml
  `,
	PreRunE: preRunPublish,
	RunE:    runPublish,
}

// preRunPublish 命令前置检查：验证参数、解析构建参数、校验语言模板
func preRunPublish(cmd *cobra.Command, args []string) error {
	applyRemoteBuilderEnvironment()

	language, _ = validateLanguageFlag(language)

	mapped, err := parseBuildArgs(buildArgs)

	if err == nil {
		buildArgMap = mapped
	}

	buildLabelMap, err = util.ParseMap(buildLabels, "build-label")

	if parallel < 1 {
		return fmt.Errorf("the --parallel flag must be great than 0")
	}

	if len(yamlFile) == 0 {
		return fmt.Errorf("--yaml or -f is required")
	}

	return err
}

// runPublish 发布命令主入口：解析配置、拉取模板、初始化构建环境、执行发布
func runPublish(cmd *cobra.Command, args []string) error {
	var services stack.Services

	if len(yamlFile) > 0 {
		parsedServices, err := stack.ParseYAMLFile(yamlFile, regex, filter, envsubst)
		if err != nil {
			return err
		}

		if parsedServices != nil {
			services = *parsedServices
		}
	}

	cwd, _ := os.Getwd()
	templatesPath := filepath.Join(cwd, TemplateDirectory)

	// 检查缺失的函数模板
	missingTemplates, err := getMissingTemplates(services.Functions, templatesPath)
	if err != nil {
		return fmt.Errorf("error accessing existing templates folder: %s", err.Error())
	}

	// 从 stack.yaml 配置拉取模板
	if len(services.StackConfiguration.TemplateConfigs) > 0 && !disableStackPull {
		if err := pullStackTemplates(missingTemplates, services.StackConfiguration.TemplateConfigs, cmd); err != nil {
			return fmt.Errorf("error pulling templates: %s", err.Error())
		}
		if len(missingTemplates) > 0 {
			log.Printf("Pulled templates: %v", missingTemplates)
		}
	} else {
		// 从官方模板仓库拉取缺失模板
		for _, missingTemplate := range missingTemplates {
			if err := runTemplateStorePull(cmd, []string{missingTemplate}); err != nil {
				return fmt.Errorf("error pulling template: %s", err.Error())
			}
		}
	}

	// 重置 QEMU 以支持跨架构构建
	if resetQemu {

		task := v2execute.ExecTask{
			Command: "docker",
			Args: []string{"run",
				"--rm",
				"--privileged",
				"multiarch/qemu-user-static",
				"--reset",
				"-p",
				"yes"},
			StreamStdio: false,
		}

		res, err := task.Execute(cmd.Context())
		if err != nil {
			return err
		}

		if res.ExitCode != 0 {
			fmt.Printf("Note: qemu-user-static only supports AMD64 at this time, see more: https://github.com/multiarch/qemu-user-static\n\n")

			return fmt.Errorf("non-zero exit code: %d, stderr: %s", res.ExitCode, res.Stderr)
		}

		fmt.Printf("Ran qemu-user-static --reset. OK.\n")
	}

	// 本地构建：创建 buildx 多架构构建节点
	if len(remoteBuilder) == 0 {
		task := v2execute.ExecTask{
			Command: "docker",
			Args: []string{"buildx",
				"create",
				"--use",
				"--name=multiarch",
				"--node=multiarch"},
			StreamStdio: false,
			Env:         []string{"DOCKER_CLI_EXPERIMENTAL=enabled"},
		}

		res, err := task.Execute(cmd.Context())
		if err != nil {
			return err
		}

		if res.ExitCode != 0 {
			return fmt.Errorf("non-zero exit code: %d, stderr: %s", res.ExitCode, res.Stderr)
		}

		fmt.Printf("Created buildx node: \"multiarch\"\n")
	}

	// 二次检查并拉取模板
	if len(services.StackConfiguration.TemplateConfigs) != 0 && !disableStackPull {
		newTemplateInfos, err := getMissingTemplates(services.Functions, "./template")
		if err != nil {
			return fmt.Errorf("already pulled templates directory has issue: %w", err)
		}

		if err := pullStackTemplates(newTemplateInfos, services.StackConfiguration.TemplateConfigs, cmd); err != nil {
			return fmt.Errorf("could not pull templates from function yaml file: %w", err)
		}
	}

	// 并行执行多架构构建与推送
	errors := publish(&services, parallel, shrinkwrap, quietBuild, mountSSH)
	if len(errors) > 0 {
		errorSummary := "Errors received during build:\n"
		for _, err := range errors {
			errorSummary = errorSummary + "- " + err.Error() + "\n"
		}
		return fmt.Errorf("%s", aec.Apply(errorSummary, aec.RedF))
	}
	return nil
}

// publish 并行工作池实现：批量构建并发布多架构函数镜像
func publish(services *stack.Services, queueDepth int, shrinkwrap, quietBuild, mountSSH bool) []error {
	startOuter := time.Now()

	errors := []error{}

	wg := sync.WaitGroup{}

	workChannel := make(chan stack.Function)

	wg.Add(queueDepth)
	for i := 0; i < queueDepth; i++ {
		go func(index int) {
			for function := range workChannel {
				start := time.Now()

				fmt.Printf(aec.YellowF.Apply("[%d] > Building %s.\n"), index, function.Name)
				if len(function.Language) == 0 {
					fmt.Println("Please provide a valid language for your function.")
				} else {
					// 合并全局与函数级别的构建参数
					combinedBuildOptions := combineBuildOpts(function.BuildOptions, buildOptions)
					combinedBuildArgMap := util.MergeMap(function.BuildArgs, buildArgMap)
					combinedExtraPaths := util.MergeSlice(services.StackConfiguration.CopyExtraPaths, copyExtra)

					// 调用核心构建逻辑：多架构构建 + 推送
					err := builder.PublishImage(function.Image,
						function.Handler,
						function.Name,
						function.Language,
						nocache,
						squash,
						shrinkwrap,
						combinedBuildArgMap,
						combinedBuildOptions,
						tagFormat,
						buildLabelMap,
						quietBuild,
						combinedExtraPaths,
						function.BuildSecrets,
						platforms,
						extraTags,
						remoteBuilder,
						payloadSecretPath,
						builderPublicKeyPath,
						forcePull,
					)

					if err != nil {
						errors = append(errors, err)
					}
				}

				duration := time.Since(start)
				fmt.Printf(aec.YellowF.Apply("[%d] < Building %s done in %1.2fs.\n"), index, function.Name, duration.Seconds())
			}

			fmt.Printf(aec.YellowF.Apply("[%d] Worker done.\n"), index)
			wg.Done()
		}(i)

	}

	// 将函数任务发送到工作通道
	for k, function := range services.Functions {
		if function.SkipBuild {
			fmt.Printf("Skipping build of: %s.\n", function.Name)
		} else {
			function.Name = k
			workChannel <- function
		}
	}

	close(workChannel)

	wg.Wait()

	duration := time.Since(startOuter)
	fmt.Printf("\n%s\n", aec.Apply(fmt.Sprintf("Total build time: %1.2fs", duration.Seconds()), aec.YellowF))
	return errors
}
