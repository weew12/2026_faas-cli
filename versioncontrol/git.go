// Package versioncontrol 提供Git版本控制相关操作封装
// 包含Git克隆、检出、初始化、SHA/分支获取等命令，用于OpenFaaS模板拉取与仓库管理
package versioncontrol

import (
	"fmt"
	"strings"

	"github.com/openfaas/faas-cli/exec"
)

// GitCloneBranch 克隆指定分支的仓库到目标目录
// 浅克隆（深度1）、禁用自动换行转换
var GitCloneBranch = &vcsCmd{
	name:   "Git",
	cmd:    "git",
	cmds:   []string{"clone {repo} {dir} --depth=1 --config core.autocrlf=false -b {refname}"},
	scheme: []string{"git", "https", "http", "git+ssh", "ssh"},
}

// GitCloneFullDepth 完整克隆仓库（非浅克隆）
// 用于后续需要检出指定SHA的场景
var GitCloneFullDepth = &vcsCmd{
	name:   "Git",
	cmd:    "git",
	cmds:   []string{"clone {repo} {dir} --config core.autocrlf=false"},
	scheme: []string{"git", "https", "http", "git+ssh", "ssh"},
}

// GitCloneDefault 克隆仓库的默认分支到目标目录
// 浅克隆、禁用自动换行转换
var GitCloneDefault = &vcsCmd{
	name:   "Git",
	cmd:    "git",
	cmds:   []string{"clone {repo} {dir} --depth=1 --config core.autocrlf=false"},
	scheme: []string{"git", "https", "http", "git+ssh", "ssh"},
}

// GitCheckout 检出仓库的指定引用（分支/标签/SHA）
var GitCheckout = &vcsCmd{
	name:   "Git",
	cmd:    "git",
	cmds:   []string{"-C {dir} checkout {refname}"},
	scheme: []string{"git", "https", "http", "git+ssh", "ssh"},
}

// GitCheckRefName 校验字符串是否为合法的Git引用名（分支/标签/SHA）
var GitCheckRefName = &vcsCmd{
	name:   "Git",
	cmd:    "git",
	cmds:   []string{"check-ref-format --allow-onelevel {refname}"},
	scheme: []string{"git", "https", "http", "git+ssh", "ssh"},
}

// GitInitRepo2_28_0 适配 Git 2.28.0+ 版本的仓库初始化命令
// 初始化仓库 → 配置用户信息 → 添加并提交所有文件
var GitInitRepo2_28_0 = &vcsCmd{
	name: "Git",
	cmd:  "git",
	cmds: []string{
		"init {dir} --initial-branch=master",
		"config core.autocrlf false",
		"config user.email \"contact@openfaas.com\"",
		"config user.name \"OpenFaaS\"",
		"add {dir}",
		"commit -m \"Test-commit\"",
	},
	scheme: []string{"git", "https", "http", "git+ssh", "ssh"},
}

// GitInitRepoClassic 兼容旧版Git的仓库初始化命令
// 无 --initial-branch 参数
var GitInitRepoClassic = &vcsCmd{
	name: "Git",
	cmd:  "git",
	cmds: []string{
		"init {dir}",
		"config core.autocrlf false",
		"config user.email \"contact@openfaas.com\"",
		"config user.name \"OpenFaaS\"",
		"add {dir}",
		"commit -m \"Test-commit\"",
	},
	scheme: []string{"git", "https", "http", "git+ssh", "ssh"},
}

// GetGitDescribe 获取当前提交的可读标识（标签+距离+短SHA）
// 格式示例：v1.2.3-5-g123456
func GetGitDescribe() string {
	getDescribeCommand := []string{"git", "describe", "--tags", "--always"}
	sha := exec.CommandWithOutput(getDescribeCommand, true)
	if strings.Contains(sha, "Not a git repository") {
		return ""
	}
	sha = strings.TrimSuffix(sha, "\n")

	return sha
}

// GetGitSHAFor 获取指定目录下的Git提交SHA值
// short=true 返回短SHA，false 返回完整SHA
func GetGitSHAFor(path string, short bool) (string, error) {
	args := []string{"-C", path, "rev-parse"}
	if short {
		args = append(args, "--short")
	}
	args = append(args, "HEAD")
	getShaCommand := []string{"git"}
	getShaCommand = append(getShaCommand, args...)
	sha := exec.CommandWithOutput(getShaCommand, true)
	if strings.Contains(sha, "Not a git repository") {
		return "", fmt.Errorf("not a git repository")
	}

	return strings.TrimSpace(sha), nil
}

// GetGitSHA 获取当前目录的短Git提交SHA值
func GetGitSHA() string {
	getShaCommand := []string{"git", "rev-parse", "--short", "HEAD"}
	sha := exec.CommandWithOutput(getShaCommand, true)
	if strings.Contains(sha, "Not a git repository") {
		return ""
	}
	sha = strings.TrimSuffix(sha, "\n")

	return sha
}

// GetGitBranch 获取当前所在的Git分支名
func GetGitBranch() string {
	getBranchCommand := []string{"git", "rev-parse", "--symbolic-full-name", "--abbrev-ref", "HEAD"}
	branch := exec.CommandWithOutput(getBranchCommand, true)
	if strings.Contains(branch, "Not a git repository") {
		return ""
	}
	branch = strings.TrimSuffix(branch, "\n")
	return branch
}
