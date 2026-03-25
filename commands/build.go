// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/morikuni/aec"
	"github.com/openfaas/faas-cli/builder"
	"github.com/openfaas/faas-cli/schema"
	"github.com/openfaas/faas-cli/util"
	"github.com/openfaas/go-sdk/stack"

	"github.com/openfaas/faas-cli/versioncontrol"
	"github.com/spf13/cobra"
)

// 命令行标志变量定义
var (
	nocache          bool
	squash           bool
	parallel         int
	shrinkwrap       bool
	buildArgs        []string
	buildArgMap      map[string]string
	buildOptions     []string
	copyExtra        []string
	tagFormat        schema.BuildFormat
	buildLabels      []string
	buildLabelMap    map[string]string
	envsubst         bool
	quietBuild       bool
	disableStackPull bool
	forcePull        bool
)

func init() {
	// 绑定通用命令行参数
	buildCmd.Flags().StringVar(&image, "image", "", "Docker image name to build")
	buildCmd.Flags().StringVar(&handler, "handler", "", "Directory with handler for function, e.g. handler.js")
	buildCmd.Flags().StringVar(&functionName, "name", "", "Name of the deployed function")
	buildCmd.Flags().StringVar(&language, "lang", "", "Programming language template")

	// 绑定 build 命令专用参数
	buildCmd.Flags().BoolVar(&nocache, "no-cache", false, "Do not use Docker's build cache")
	buildCmd.Flags().BoolVar(&squash, "squash", false, `Use Docker's squash flag for smaller images [experimental] `)
	buildCmd.Flags().IntVar(&parallel, "parallel", 1, "Build in parallel to depth specified.")
	buildCmd.Flags().BoolVar(&shrinkwrap, "shrinkwrap", false, "Just write files to ./build/ folder for shrink-wrapping")
	buildCmd.Flags().StringArrayVarP(&buildArgs, "build-arg", "b", []string{}, "Add a build-arg for Docker (KEY=VALUE)")
	buildCmd.Flags().StringArrayVarP(&buildOptions, "build-option", "o", []string{}, "Set a build option, e.g. dev")
	buildCmd.Flags().Var(&tagFormat, "tag", "Override latest tag on function Docker image, accepts 'digest', 'sha', 'branch', or 'describe', or 'latest'")
	buildCmd.Flags().StringArrayVar(&buildLabels, "build-label", []string{}, "Add a label for Docker image (LABEL=VALUE)")
	buildCmd.Flags().StringArrayVar(&copyExtra, "copy-extra", []string{}, "Extra paths that will be copied into the function build context")
	buildCmd.Flags().BoolVar(&envsubst, "envsubst", true, "Substitute environment variables in stack.yaml file")
	buildCmd.Flags().BoolVar(&quietBuild, "quiet", false, "Perform a quiet build, without showing output from Docker")
	buildCmd.Flags().BoolVar(&disableStackPull, "disable-stack-pull", false, "Disables the template configuration in the stack.yaml")
	buildCmd.Flags().BoolVar(&forcePull, "pull", false, "Force a re-pull of base images in template during build, useful for publishing images")

	buildCmd.Flags().BoolVar(&pullDebug, "debug", false, "Enable debug output when pulling templates")
	buildCmd.Flags().BoolVar(&overwrite, "overwrite", true, "Overwrite existing templates from the template repository")

	// 配置命令行自动补全
	_ = buildCmd.Flags().SetAnnotation("handler", cobra.BashCompSubdirsInDir, []string{})

	// 将 build 命令添加到根命令
	faasCmd.AddCommand(buildCmd)
}

// buildCmd 构建 OpenFaaS 函数容器的主命令
var buildCmd = &cobra.Command{
	Use: `build -f YAML_FILE [--no-cache] [--squash]
  faas-cli build --image IMAGE_NAME
                 --handler HANDLER_DIR
                 --name FUNCTION_NAME
                 [--lang <ruby|python|python3|node|csharp|dockerfile>]
                 [--no-cache] [--squash]
                 [--regex "REGEX"]
                 [--filter "WILDCARD"]
                 [--parallel PARALLEL_DEPTH]
                 [--build-arg KEY=VALUE]
                 [--build-option VALUE]
                 [--copy-extra PATH]
                 [--tag <digest|sha|branch|describe>]
				 [--forcePull]`,
	Short: "Builds OpenFaaS function containers",
	Long: `Builds OpenFaaS function containers either via the supplied YAML config using
the "--yaml" flag (which may contain multiple function definitions), or directly
via flags.`,
	Example: `  faas-cli build -f https://domain/path/myfunctions.yml
  faas-cli build -f functions.yaml
  faas-cli build --no-cache --build-arg NPM_VERSION=0.2.2
  faas-cli build --build-option dev
  faas-cli build --tag sha
  faas-cli build --parallel 4
  faas-cli build --filter "*gif*"
  faas-cli build --regex "fn[0-9]_.*"
  faas-cli build --image=my_image --lang=python --handler=/path/to/fn/ \
                 --name=my_fn --squash
  faas-cli build --build-label org.label-schema.label-name="value"`,
	PreRunE: preRunBuild,
	RunE:    runBuild,
}

// preRunBuild 构建前校验参数
func preRunBuild(cmd *cobra.Command, args []string) error {
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

	return err
}

// parseBuildArgs 解析 --build-arg 参数为键值对 map
func parseBuildArgs(args []string) (map[string]string, error) {
	mapped := make(map[string]string)

	for _, kvp := range args {
		index := strings.Index(kvp, "=")
		if index == -1 {
			return nil, fmt.Errorf("each build-arg must take the form key=value")
		}

		values := []string{kvp[0:index], kvp[index+1:]}

		k := strings.TrimSpace(values[0])
		v := strings.TrimSpace(values[1])

		if len(k) == 0 {
			return nil, fmt.Errorf("build-arg must have a non-empty key")
		}
		if len(v) == 0 {
			return nil, fmt.Errorf("build-arg must have a non-empty value")
		}

		if k == builder.AdditionalPackageBuildArg && len(mapped[k]) > 0 {
			mapped[k] = mapped[k] + " " + v
		} else {
			mapped[k] = v
		}
	}

	return mapped, nil
}

// runBuild 执行构建主逻辑
func runBuild(cmd *cobra.Command, args []string) error {

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

	// 自动拉取缺失的函数模板
	if len(services.StackConfiguration.TemplateConfigs) > 0 && !disableStackPull {
		missingTemplates, err := getMissingTemplates(services.Functions, templatesPath)
		if err != nil {
			return fmt.Errorf("error accessing existing templates folder: %s", err.Error())
		}

		if err := pullStackTemplates(missingTemplates, services.StackConfiguration.TemplateConfigs, cmd); err != nil {
			return fmt.Errorf("error pulling templates: %s", err.Error())
		}
		if len(missingTemplates) > 0 {
			log.Printf("Pulled templates: %v", missingTemplates)
		}

	} else {

		// 从模板仓库拉取缺失模板
		missingTemplates, err := getMissingTemplates(services.Functions, templatesPath)
		if err != nil {
			return fmt.Errorf("error accessing existing templates folder: %s", err.Error())
		}

		for _, missingTemplate := range missingTemplates {

			if err := runTemplateStorePull(cmd, []string{missingTemplate}); err != nil {
				return fmt.Errorf("error pulling template: %s", err.Error())
			}
		}

	}

	// 不使用 YAML，直接用命令行参数构建单个函数
	if len(services.Functions) == 0 {
		if len(image) == 0 {
			return fmt.Errorf("please provide a valid --image name for your Docker image")
		}
		if len(handler) == 0 {
			return fmt.Errorf("please provide the full path to your function's handler")
		}
		if len(functionName) == 0 {
			return fmt.Errorf("please provide the deployed --name of your function")
		}

		if err := builder.BuildImage(image,
			handler,
			functionName,
			language,
			nocache,
			squash,
			shrinkwrap,
			buildArgMap,
			buildOptions,
			tagFormat,
			buildLabelMap,
			quietBuild,
			copyExtra,
			nil,
			remoteBuilder,
			payloadSecretPath,
			builderPublicKeyPath,
			forcePull,
		); err != nil {
			return err
		}

		return nil
	}

	// 批量构建 stack.yaml 中的所有函数
	errors := build(&services, parallel, shrinkwrap, quietBuild)
	if len(errors) > 0 {
		errorSummary := "Errors received during build:\n"
		for _, err := range errors {
			errorSummary = errorSummary + "- " + err.Error() + "\n"
		}
		return fmt.Errorf("%s", aec.Apply(errorSummary, aec.RedF))
	}

	return nil
}

// build 并行构建多个函数
func build(services *stack.Services, queueDepth int, shrinkwrap, quietBuild bool) []error {
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
					// 合并 YAML 与命令行中的构建参数
					combinedBuildOptions := combineBuildOpts(function.BuildOptions, buildOptions)
					combinedBuildArgMap := util.MergeMap(function.BuildArgs, buildArgMap)
					combinedExtraPaths := util.MergeSlice(services.StackConfiguration.CopyExtraPaths, copyExtra)
					// 执行构建
					err := builder.BuildImage(function.Image,
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

	// 分发任务到工作协程
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

// pullTemplates 从 Git 仓库拉取模板
func pullTemplates(templateURL, templateName string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("can't get current working directory: %s", err)
	}

	templatePath := filepath.Join(cwd, TemplateDirectory)

	if _, err := os.Stat(templatePath); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("No templates found in current directory.\n")

			templateURL, refName := versioncontrol.ParsePinnedRemote(templateURL)
			if err := fetchTemplates(templateURL, refName, templateName, overwrite); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

// combineBuildOpts 合并 YAML 和命令行中的 build-options
func combineBuildOpts(YAMLBuildOpts []string, buildFlagBuildOpts []string) []string {
	return util.MergeSlice(YAMLBuildOpts, buildFlagBuildOpts)
}
