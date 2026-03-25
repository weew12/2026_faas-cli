// Package commands 实现 OpenFaaS CLI 插件管理命令
// 本文件实现 plugin get 子命令：从容器镜像仓库下载并安装 CLI 插件
package commands

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/alexellis/arkade/pkg/archive"
	"github.com/alexellis/arkade/pkg/env"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/spf13/cobra"
)

// 全局命令行参数
var (
	pluginRegistry string // 插件镜像仓库地址
	clientOS       string // 客户端操作系统（自动检测或手动指定）
	clientArch     string // 客户端架构（自动检测或手动指定）
	tag            string // 插件版本号
	pluginPath     string // 插件安装路径
)

// init 初始化 plugin get 命令并注册到 plugin 根命令
func init() {
	pluginGetCmd := &cobra.Command{
		Use:   "get",
		Short: "Get a plugin",
		Long: `Download and extract a plugin for faas-cli from a container
registry`,
		Example: `# Download a plugin by name:
faas-cli plugin get NAME

# Give a version
faas-cli plugin get NAME --version 0.0.1

# Give an explicit OS and architecture
faas-cli plugin get NAME --arch armhf --os linux

# Use a custom registry
faas-cli plugin get NAME --registry ghcr.io/openfaasltd`,
		RunE: runPluginGetCmd,
	}

	// 注册命令行标志
	pluginGetCmd.Flags().StringVar(&pluginRegistry, "registry", "ghcr.io/openfaasltd", "The registry to pull the plugin from")
	pluginGetCmd.Flags().StringVar(&clientArch, "arch", "", "The architecture to pull the plugin for, give a value or leave blank for auto-detection")
	pluginGetCmd.Flags().StringVar(&clientOS, "os", "", "The OS to pull the plugin for, give a value or leave blank for auto-detection")
	pluginGetCmd.Flags().StringVar(&tag, "version", "latest", "Version or SHA for plugin")
	pluginGetCmd.Flags().StringVar(&pluginPath, "path", "$HOME/.openfaas/plugins", "The path for the plugin")

	pluginGetCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	// 将 get 子命令添加到 plugin 根命令
	pluginCmd.AddCommand(pluginGetCmd)
}

// runPluginGetCmd 插件下载安装的核心执行逻辑
func runPluginGetCmd(cmd *cobra.Command, args []string) error {
	// 必须传入插件名称
	if len(args) < 1 {
		return fmt.Errorf("please provide the name of the plugin")
	}

	// 必须指定版本
	if len(tag) == 0 {
		return fmt.Errorf("please provide the version of the plugin or \"latest\"")
	}

	// 自动检测客户端架构与系统
	arch, operatingSystem := getClientArch()

	// 未手动指定则使用自动检测值
	if len(clientArch) == 0 {
		clientArch = arch
	}
	if len(clientOS) == 0 {
		clientOS = operatingSystem
	}

	st := time.Now()                                                // 记录下载开始时间
	pluginName := args[0]                                           // 插件名称
	src := fmt.Sprintf("%s/%s:%s", pluginRegistry, pluginName, tag) // 完整镜像地址

	// 日志输出
	if verbose {
		fmt.Printf("Fetching plugin: %s %s for: %s/%s\n", pluginName, src, clientOS, clientArch)
	} else {
		fmt.Printf("Fetching plugin: %s\n", pluginName)
	}

	var pluginDir string

	// 确定插件安装目录
	if cmd.Flags().Changed("path") {
		// 替换 $HOME 环境变量
		pluginPath = strings.ReplaceAll(pluginPath, "$HOME", os.Getenv("HOME"))
		pluginDir = pluginPath
	} else {
		// 默认目录：~/.openfaas/plugins
		if runtime.GOOS == "windows" {
			pluginDir = os.Expand("$HOMEPATH/.openfaas/plugins", os.Getenv)
		} else {
			pluginDir = os.ExpandEnv("$HOME/.openfaas/plugins")
		}
	}

	// 创建插件目录（不存在则创建）
	if _, err := os.Stat(pluginDir); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(pluginDir, 0755); err != nil && !os.IsExist(err) {
			return fmt.Errorf("failed to create plugin directory %s: %w", pluginDir, err)
		}
	}

	// 创建临时 tar 文件保存镜像
	tmpTar := path.Join(os.TempDir(), pluginName+".tar")
	f, err := os.Create(tmpTar)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", tmpTar, err)
	}
	defer f.Close()

	var img v1.Image

	// 规范架构名称
	downloadArch, downloadOS := getDownloadArch(clientArch, clientOS)

	// 从容器仓库拉取插件镜像
	img, err = crane.Pull(src, crane.WithPlatform(&v1.Platform{Architecture: downloadArch, OS: downloadOS}))
	if err != nil {
		return fmt.Errorf("pulling %s: %w", src, err)
	}

	// 导出镜像为 tar 包
	if err := crane.Export(img, f); err != nil {
		return fmt.Errorf("exporting %s: %w", src, err)
	}

	if verbose {
		fmt.Printf("Wrote OCI filesystem to: %s\n", tmpTar)
	}

	// 打开临时 tar 文件
	tarFile, err := os.Open(tmpTar)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", tmpTar, err)
	}
	defer tarFile.Close()

	if verbose {
		fmt.Printf("Writing %q\n", path.Join(pluginDir, pluginName))
	}

	// 下载完成后删除临时文件
	defer os.Remove(tmpTar)

	// 解压插件到目标目录
	gzipped := false
	if err := archive.Untar(tarFile, pluginDir, gzipped, true); err != nil {
		return fmt.Errorf("failed to untar %s: %w", tmpTar, err)
	}

	// Windows 系统需要添加 .exe 后缀才能执行
	if runtime.GOOS == "windows" {
		pluginPath := path.Join(pluginDir, pluginName)
		err := os.Rename(pluginPath, fmt.Sprintf("%s.exe", pluginPath))
		if err != nil {
			return fmt.Errorf("failed to move plugin %w", err)
		}
	}

	// 完成提示
	if cmd.Flags().Changed("path") {
		fmt.Printf("Wrote: %s (%s/%s) in (%s)\n", path.Join(pluginPath, pluginName), clientOS, clientArch, formatDownloadDuration(time.Since(st)))
	} else {
		fmt.Printf("Downloaded in (%s)\n\nUsage:\n  faas-cli %s\n", formatDownloadDuration(time.Since(st)), pluginName)
	}
	return nil
}

// formatDownloadDuration 格式化下载耗时显示
func formatDownloadDuration(downloadTime time.Duration) string {
	if downloadTime < time.Millisecond {
		return "<1ms"
	}
	return downloadTime.Round(time.Millisecond).String()
}

// getClientArch 获取客户端架构与操作系统
func getClientArch() (arch string, os string) {
	if runtime.GOOS == "windows" {
		return runtime.GOARCH, runtime.GOOS
	}
	return env.GetClientArch()
}

// getDownloadArch 规范架构名称（兼容别名）
func getDownloadArch(clientArch, clientOS string) (arch string, os string) {
	downloadArch := strings.ToLower(clientArch)
	downloadOS := strings.ToLower(clientOS)

	// 架构别名标准化
	if downloadArch == "x86_64" {
		downloadArch = "amd64"
	} else if downloadArch == "aarch64" {
		downloadArch = "arm64"
	}

	return downloadArch, downloadOS
}
