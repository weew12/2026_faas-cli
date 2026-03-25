// Package versioncontrol 是 go/internal/get/vcs 的简化精简版本
// 专为 OpenFaaS 模板拉取场景设计，用于执行临时 Git 克隆等版本控制操作
package versioncontrol

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// vcsCmd 表示一个版本控制系统命令（如 Git）
type vcsCmd struct {
	name   string   // 版本控制系统名称
	cmd    string   // 要调用的命令行工具名称（如 git）
	cmds   []string // 要执行的命令序列
	scheme []string // 该命令支持的 URI 协议（如 http、https、ssh）
}

// Invoke 执行版本控制命令
// 将命令中的变量替换为传入的参数，并依次执行所有命令
func (v *vcsCmd) Invoke(dir string, args map[string]string) error {
	for _, cmd := range v.cmds {
		if _, err := v.run(dir, cmd, args, true); err != nil {
			return err
		}
	}
	return nil
}

// run 执行具体的外部命令
// dir：执行命令的工作目录
// cmdline：命令行字符串
// keyval：变量替换键值对
// verbose：是否输出错误详情
func (v *vcsCmd) run(dir string, cmdline string, keyval map[string]string, verbose bool) ([]byte, error) {
	args := strings.Fields(cmdline)
	// 替换命令中的 {key} 格式变量
	for i, arg := range args {
		args[i] = replaceVars(keyval, arg)
	}

	// 检查命令是否存在
	_, err := exec.LookPath(v.cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "missing %s command", v.name)
		return nil, err
	}

	// 调试模式输出命令
	if os.Getenv("FAAS_DEBUG") == "1" {
		log.Printf("[git] %s %s", v.cmd, strings.Join(args, " "))
	}

	// 构建并执行命令
	cmd := exec.Command(v.cmd, args...)
	cmd.Dir = dir
	cmd.Env = envWithPWD(cmd.Dir)

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err = cmd.Run()
	out := buf.Bytes()
	if err != nil {
		if verbose {
			os.Stderr.Write(out)
		}
		return out, err
	}
	return out, nil
}

// replaceVars 替换字符串中的 {key} 变量
// 将 s 中的 {k} 替换为 vars[k]
func replaceVars(vars map[string]string, s string) string {
	for key, value := range vars {
		s = strings.Replace(s, "{"+key+"}", value, -1)
	}
	return s
}

// envWithPWD 更新环境变量中的 PWD 为指定目录
// 确保命令执行时的工作目录与 PWD 一致
func envWithPWD(dir string) []string {
	env := os.Environ()
	updated := false
	for i, envVar := range env {
		if strings.HasPrefix(envVar, "PWD") {
			env[i] = "PWD=" + dir
			updated = true
		}
	}

	if !updated {
		env = append(env, "PWD="+dir)
	}
	return env
}
