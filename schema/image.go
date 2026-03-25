package schema

import (
	"fmt"
	"strings"
)

// BuildFormat 定义构建过程中使用的 Docker 镜像标签格式
type BuildFormat int

// DefaultFormat 使用 YAML 文件中定义的格式，或追加 :latest
const DefaultFormat BuildFormat = 0

// SHAFormat 使用 "latest-<sha>" 作为镜像标签
const SHAFormat BuildFormat = 1

// BranchAndSHAFormat 使用 "latest-<branch>-<sha>" 作为镜像标签
const BranchAndSHAFormat BuildFormat = 2

// DescribeFormat 使用 git-describe 输出作为镜像标签
const DescribeFormat BuildFormat = 3

// DigestFormat 使用摘要格式作为镜像标签
const DigestFormat BuildFormat = 4

// Type 实现 pflag.Value 接口
func (i *BuildFormat) Type() string {
	return "string"
}

// String 实现 fmt.Stringer 接口
func (i *BuildFormat) String() string {
	if i == nil {
		return "latest"
	}

	switch *i {
	case DefaultFormat:
		return "latest"
	case SHAFormat:
		return "sha"
	case BranchAndSHAFormat:
		return "branch"
	case DescribeFormat:
		return "describe"
	default:
		return "latest"
	}
}

// Set 实现 pflag.Value 接口
func (i *BuildFormat) Set(value string) error {
	switch strings.ToLower(value) {
	case "", "default", "latest":
		*i = DefaultFormat
	case "sha":
		*i = SHAFormat
	case "branch":
		*i = BranchAndSHAFormat
	case "describe":
		*i = DescribeFormat
	case "digest":
		*i = DigestFormat
	default:
		return fmt.Errorf("unknown image tag format: '%s'", value)
	}
	return nil
}

// BuildImageName 为构建、推送、部署生成 Docker 镜像标签
func BuildImageName(format BuildFormat, image string, version string, branch string) string {
	imageVal := image
	splitImage := strings.Split(image, "/")
	if !strings.Contains(splitImage[len(splitImage)-1], ":") {
		imageVal += ":latest"
	}

	switch format {
	case SHAFormat:
		return imageVal + "-" + version
	case BranchAndSHAFormat:
		return imageVal + "-" + branch + "-" + version
	case DescribeFormat:
		return imageVal + "-" + version
	case DigestFormat:

		if lastIndex := strings.LastIndex(imageVal, ":"); lastIndex > -1 {
			baseImage := imageVal[:lastIndex]
			return fmt.Sprintf("%s:%s", baseImage, version)
		} else {
			return imageVal + "-" + version
		}
	default:
		return imageVal
	}
}
