// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 命令行工具的所有命令逻辑
// 本文件实现 push 命令，用于将本地构建好的函数镜像推送到远程容器仓库
package commands

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/openfaas/faas-cli/exec"

	"github.com/morikuni/aec"
	"github.com/openfaas/faas-cli/builder"
	"github.com/openfaas/faas-cli/schema"
	"github.com/openfaas/go-sdk/stack"
	"github.com/spf13/cobra"
)

// init 初始化 push 命令，注册命令行参数并添加到主命令 faasCmd
func init() {
	faasCmd.AddCommand(pushCmd)

	pushCmd.Flags().IntVar(&parallel, "parallel", 1, "Push images in parallel to depth specified.")
	pushCmd.Flags().Var(&tagFormat, "tag", "Override latest tag on function Docker image, accepts 'digest', 'latest', 'sha', 'branch', 'describe'")
	pushCmd.Flags().BoolVar(&envsubst, "envsubst", true, "Substitute environment variables in stack.yaml file")
	pushCmd.Flags().BoolVar(&quietBuild, "quiet", false, "Perform a quiet build, without showing output from Docker")
}

// pushCmd 将本地函数容器镜像推送到远程仓库（如 Docker Hub、阿里云、ECR 等）
// 必须先通过 build 命令构建本地镜像，支持批量、并行、自定义标签
var pushCmd = &cobra.Command{
	Use:   `push -f YAML_FILE [--regex "REGEX"] [--filter "WILDCARD"] [--parallel] [--tag <sha|branch>]`,
	Short: "Push OpenFaaS functions to remote registry (Docker Hub)",
	Long: `Pushes the OpenFaaS function container image(s) defined in the supplied YAML
config to a remote repository.

These container images must already be present in your local image cache.`,

	Example: `  faas-cli push -f https://domain/path/myfunctions.yml
  faas-cli push -f stack.yaml
  faas-cli push -f stack.yaml --parallel 4
  faas-cli push -f stack.yaml --filter "*gif*"
  faas-cli push -f stack.yaml --regex "fn[0-9]_.*"
  faas-cli push -f stack.yaml --tag sha
  faas-cli push -f stack.yaml --tag branch
  faas-cli push -f stack.yaml --tag describe`,
	RunE: runPush,
}

// runPush 执行推送逻辑的入口函数
// 1. 解析 stack.yaml
// 2. 验证镜像名称合法性
// 3. 调用 pushStack 批量推送
func runPush(cmd *cobra.Command, args []string) error {

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

	if len(services.Functions) > 0 {
		// 验证所有函数镜像是否包含仓库用户名/地址（必须符合 user/image 格式）
		invalidImages := validateImages(services.Functions)
		if len(invalidImages) > 0 {
			imageList := strings.Join(invalidImages, "\n- ")
			return fmt.Errorf(`
Unable to push one or more of your functions to Docker Hub:
- %s

You must provide a username or registry prefix to the Function's image such as user1/function1`, imageList)
		}

		// 并行推送所有函数镜像
		pushStack(&services, parallel, tagFormat)
	} else {
		return fmt.Errorf("you must supply a valid YAML file")
	}
	return nil
}

// pushImage 执行 docker push 命令推送单个镜像
func pushImage(image string, quietBuild bool) {
	args := []string{"docker", "push", image}
	if quietBuild {
		args = append(args, "--quiet")
	}

	exec.Command("./", args)
}

// pushStack 使用工作池并行推送多个函数镜像
func pushStack(services *stack.Services, queueDepth int, tagFormat schema.BuildFormat) {
	wg := sync.WaitGroup{}

	workChannel := make(chan stack.Function)

	// 启动指定数量的并行工作协程
	wg.Add(queueDepth)
	for i := 0; i < queueDepth; i++ {
		go func(index int) {
			for function := range workChannel {

				// 获取版本信息用于生成镜像标签
				branch, sha, err := builder.GetImageTagValues(tagFormat, function.Handler)
				if err != nil {
					log.Printf("Error formatting image tag, defaulting to default format: %s", err.Error())
					tagFormat = schema.DefaultFormat
				}

				// 根据 tag 格式生成最终镜像名称
				imageName := schema.BuildImageName(tagFormat, function.Image, sha, branch)

				fmt.Printf(aec.YellowF.Apply("[%d] > Pushing %s [%s]\n"), index, function.Name, imageName)
				if len(function.Image) == 0 {
					fmt.Println("Please provide a valid Image value in the YAML file.")
				} else if function.SkipBuild {
					fmt.Printf("Skipping %s\n", function.Name)
				} else {
					// 执行推送
					pushImage(imageName, quietBuild)
					fmt.Printf(aec.YellowF.Apply("[%d] < Pushing %s [%s] done.\n"), index, function.Name, imageName)
				}
			}

			fmt.Printf(aec.YellowF.Apply("[%d] Worker done.\n"), index)
			wg.Done()
		}(i)
	}

	// 将所有函数发送到工作通道
	for k, function := range services.Functions {
		function.Name = k
		workChannel <- function
	}

	close(workChannel)

	wg.Wait()

}

// validateImages 验证函数镜像名称是否合法
// 必须包含 / 分隔符，例如 username/function 或 registry/name
func validateImages(functions map[string]stack.Function) []string {
	invalidImages := []string{}

	for name, function := range functions {

		if !function.SkipBuild && !strings.Contains(function.Image, `/`) {
			invalidImages = append(invalidImages, name)
		}
	}
	return invalidImages
}
