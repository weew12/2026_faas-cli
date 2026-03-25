// Package version 提供程序版本信息管理
// 包含版本号、Git提交记录、构建版本获取等功能
package version

var (
	// Version 程序版本号（编译时通过 ldflags 注入）
	Version string
	// GitCommit 当前构建对应的 Git 提交哈希值
	GitCommit string
)

// BuildVersion 获取程序构建版本
// 当 Version 未设置时，返回 "dev" 表示开发版本
func BuildVersion() string {
	if len(Version) == 0 {
		return "dev"
	}
	return Version
}
