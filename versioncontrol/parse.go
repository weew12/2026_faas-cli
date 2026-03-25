// Package versioncontrol 提供 Git 仓库地址解析、验证工具
// 用于校验 Git 远程地址格式、解析带版本锁定的仓库地址
package versioncontrol

import (
	"regexp"
	"strings"
)

const (
	pinCharacter             = `#`                                                         // 版本锁定分隔符
	gitRemoteRegexpStr       = `(git|ssh|https?|git@[-\w.]+):(\/\/)?([^#]*?(?:\.git)?\/?)` // Git 地址正则基础
	gitPinnedRemoteRegexpStr = gitRemoteRegexpStr + pinCharacter + `[-\/\d\w._]+$`         // 带 # 锁定版本的 Git 地址正则
	gitRemoteRepoRegexpStr   = gitRemoteRegexpStr + `$`                                    // 标准 Git 地址正则
)

var (
	gitPinnedRegexp = regexp.MustCompile(gitPinnedRemoteRegexpStr) // 匹配带版本锁定的 Git 地址
	gitRemoteRegexp = regexp.MustCompile(gitRemoteRepoRegexpStr)   // 匹配标准 Git 地址
)

// IsGitRemote 验证输入字符串是否为合法的 Git 远程仓库地址
// 支持 git/ssh/http/https/git@ 格式
func IsGitRemote(repoURL string) bool {
	// 复制正则对象避免多 goroutine 锁竞争
	return gitRemoteRegexp.Copy().MatchString(repoURL)
}

// IsPinnedGitRemote 验证输入是否为带版本锁定的 Git 地址
// 格式：仓库地址#分支/标签/SHA
func IsPinnedGitRemote(repoURL string) bool {
	// 复制正则对象避免多 goroutine 锁竞争
	return gitPinnedRegexp.Copy().MatchString(repoURL)
}

// ParsePinnedRemote 解析带 # 版本锁定的 Git 地址
// 将地址拆分为 仓库地址 + 引用名（分支/标签/SHA）
// 返回：remoteURL 仓库地址, refName 引用名
func ParsePinnedRemote(repoURL string) (remoteURL, refName string) {
	// 默认引用名为空，使用仓库默认分支
	refName = ""
	remoteURL = repoURL

	// 非锁定格式直接返回
	if !IsPinnedGitRemote(repoURL) {
		return remoteURL, refName
	}

	// 按 # 分割仓库地址与引用
	atIndex := strings.LastIndex(repoURL, pinCharacter)
	if atIndex > 0 {
		remoteURL, refName, _ = strings.Cut(repoURL, pinCharacter)
	}

	// 验证分割后的基础地址是否合法
	if !IsGitRemote(remoteURL) {
		return remoteURL, refName
	}

	return remoteURL, refName
}
