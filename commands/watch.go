// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// Package commands 实现 OpenFaaS CLI 的所有命令行功能
// 本文件实现文件热重载/监听功能，监控函数代码与配置文件变更并自动重建部署
package commands

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/openfaas/go-sdk/stack"
	"github.com/spf13/cobra"
)

// watchLoop 监听函数处理器文件与 stack.yaml 配置文件的变更
// 当检测到文件变化时，通过防抖处理触发 onChange 回调函数执行重建/部署
// 参数：
//
//	cmd - cobra 命令对象
//	args - 命令行参数
//	onChange - 文件变化后执行的回调函数（构建/推送/部署函数）
//
// 返回：监听过程中发生的错误
func watchLoop(cmd *cobra.Command, args []string, onChange func(cmd *cobra.Command, args []string, ctx context.Context) error) error {
	// 解析 stack.yaml 配置文件
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

	// 收集所有需要监听的函数名称
	fnNames := []string{}
	for name := range services.Functions {
		fnNames = append(fnNames, name)
	}

	fmt.Printf("[Watch] monitoring %d functions: %s\n", len(fnNames), strings.Join(fnNames, ", "))

	// 初始化上下文取消器
	canceller := Cancel{}

	// 创建文件系统监听器
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// 加载 .gitignore 规则，过滤不需要监听的文件
	patterns, err := ignorePatterns()
	if err != nil {
		return err
	}
	matcher := gitignore.NewMatcher(patterns)

	// 获取当前工作目录并监听 stack.yaml
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	yamlPath := path.Join(cwd, yamlFile)

	debug := os.Getenv("FAAS_DEBUG")
	if debug == "1" {
		fmt.Printf("[Watch] added: %s\n", yamlPath)
	}
	watcher.Add(yamlPath)

	// 建立函数名与对应代码目录的映射关系，用于快速定位变更所属函数
	handlerMap := make(map[string]string)
	for serviceName, service := range services.Functions {
		handlerMap[serviceName] = path.Join(cwd, service.Handler)
		handlerFullPath := path.Join(cwd, service.Handler)

		// 递归添加函数代码目录到监听器
		if err := addPath(watcher, handlerFullPath); err != nil {
			return err
		}
	}

	// 监听系统中断信号（Ctrl+C / kill）
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

	// 创建防抖处理器，1.5s 内多次变化合并为一次触发，避免频繁构建
	bounce := debounce.New(1500 * time.Millisecond)

	// 首次启动执行一次构建
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		canceller.Set(ctx, cancel)

		if err := onChange(cmd, args, ctx); err != nil {
			fmt.Println("Error rebuilding: ", err)
			os.Exit(1)
		}
	}()

	log.Printf("[Watch] Started")
	// 主循环：处理文件事件、错误、系统信号
	for {
		select {
		// 处理文件变更事件
		case event, ok := <-watcher.Events:
			if !ok {
				return fmt.Errorf("watcher's Events channel is closed")
			}

			// 调试模式：打印事件详情
			if debug == "1" {
				log.Printf("[Watch] event: %s on: %s", strings.ToLower(event.Op.String()), event.Name)
			}

			// 忽略编辑器临时文件
			if strings.HasSuffix(event.Name, ".swp") || strings.HasSuffix(event.Name, "~") || strings.HasSuffix(event.Name, ".swx") {
				continue
			}

			// 只处理写入、创建、删除、重命名事件
			if event.Op == fsnotify.Write || event.Op == fsnotify.Create || event.Op == fsnotify.Remove || event.Op == fsnotify.Rename {
				info, err := os.Stat(event.Name)
				if err != nil {
					continue
				}

				// 根据 .gitignore 规则判断是否忽略该文件
				ignore := false
				if matcher.Match(strings.Split(event.Name, "/"), info.IsDir()) {
					ignore = true
				}

				// 定位变更文件所属的函数
				target := ""
				// 精确匹配
				for fnName, fnPath := range handlerMap {
					if event.Name == fnPath {
						target = fnName
					}
				}
				// 模糊匹配（子目录/子文件）
				if target == "" {
					for fnName, fnPath := range handlerMap {
						if strings.HasPrefix(event.Name, fnPath) {
							target = fnName
						}
					}
				}

				// 新增函数子目录，自动加入监听
				if event.Op == fsnotify.Create && info.IsDir() && target != "" {
					if err := addPath(watcher, event.Name); err != nil {
						return err
					}
				}

				// 非忽略文件，触发重建
				if !ignore {
					if target == "" {
						fmt.Printf("[Watch] Rebuilding %d functions reason: %s to %s\n", len(fnNames), strings.ToLower(event.Op.String()), event.Name)
					} else {
						fmt.Printf("[Watch] Reloading %s reason: %s %s\n", target, strings.ToLower(event.Op.String()), event.Name)
					}

					// 防抖执行：取消上一次任务，重新构建
					bounce(func() {
						log.Printf("[Watch] Cancelling")
						canceller.Cancel()
						log.Printf("[Watch] Cancelled")

						// 创建新上下文
						ctx, cancel := context.WithCancel(context.Background())
						canceller.Set(ctx, cancel)

						// 设置过滤目标函数，无匹配则重建所有函数
						filter = target

						// 异步执行构建部署
						go func() {
							if err := onChange(cmd, args, ctx); err != nil {
								fmt.Println("Error rebuilding: ", err)
								os.Exit(1)
							}
						}()
					})
				}
			}

		// 处理监听器错误
		case err, ok := <-watcher.Errors:
			if !ok {
				return fmt.Errorf("watcher's Errors channel is closed")
			}
			return err

		// 处理系统退出信号
		case <-signalChannel:
			watcher.Close()
			return nil
		}
	}

	return nil
}

// addPath 递归遍历目录，并将所有子目录添加到文件监听器
// 参数：
//
//	watcher - fsnotify 监听器实例
//	rootPath - 需要递归监听的根目录
//
// 返回：添加过程中发生的错误
func addPath(watcher *fsnotify.Watcher, rootPath string) error {
	debug := os.Getenv("FAAS_DEBUG")

	// 遍历目录，只监听文件夹（fsnotify 仅支持监听目录）
	return filepath.WalkDir(rootPath, func(subPath string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// 只监听目录
		if d.IsDir() {
			if err := watcher.Add(subPath); err != nil {
				return fmt.Errorf("unable to watch %s: %s", subPath, err)
			}

			// 调试模式打印已监听目录
			if debug == "1" {
				fmt.Printf("[Watch] added: %s\n", subPath)
			}
		}

		return nil
	})
}

// Cancel 用于在闭包/协程之间传递上下文和取消函数
// 实现安全的上下文取消管理，避免并发冲突
type Cancel struct {
	cancel context.CancelFunc // 上下文取消函数
	ctx    context.Context    // 监听/构建任务上下文
}

// Set 设置上下文和取消函数
func (c *Cancel) Set(ctx context.Context, cancel context.CancelFunc) {
	c.cancel = cancel
	c.ctx = ctx
}

// Cancel 执行上下文取消操作，终止正在运行的任务
func (c *Cancel) Cancel() {
	if c.cancel != nil {
		c.cancel()
	}
}
