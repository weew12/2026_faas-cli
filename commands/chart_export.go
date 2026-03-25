// Copyright (c) OpenFaaS Author(s) 2024. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"
)

// 命令行参数变量
var (
	chartExportOutput    string   // 输出目录
	chartExportValues    []string // values 文件路径
	chartExportSet       []string // 手动设置的 value
	chartExportCRDs      bool     // 是否包含 CRDs
	chartExportNamespace string   // Kubernetes 命名空间
	chartExportRelease   string   // Helm release 名称
)

// chartExportCmd 导出 Helm chart 并拆分为单个资源 YAML 的命令
var chartExportCmd = &cobra.Command{
	Use:   `export [CHART_NAME] [flags]`,
	Short: "Render a Helm chart and export each resource as a separate YAML file",
	Long: `Renders a Helm chart using "helm template" and splits the output into
individual YAML files, organised into folders by resource kind.

CustomResourceDefinitions are prefixed with 00_ so that they sort first
when applied with "kubectl apply -f".

CHART_NAME is optional and defaults to "openfaas". It maps to
"chart/<CHART_NAME>" relative to the current directory.`,
	Example: `  # Export the openfaas chart with default values
  faas-cli chart export

  # Export with pro values
  faas-cli chart export --values chart/openfaas/values-pro.yaml

  # Export kafka-connector chart to a custom directory
  faas-cli chart export kafka-connector -o ./rendered

  # Export without CRDs
  faas-cli chart export --crds=false

  # Export with value overrides
  faas-cli chart export --values chart/openfaas/values-pro.yaml --set openfaasPro=true`,
	RunE:    runChartExport,
	PreRunE: preRunChartExport,
}

func init() {
	// 绑定命令行参数
	chartExportCmd.Flags().StringVarP(&chartExportOutput, "output", "o", "./yaml", "Output directory for rendered YAML files")
	chartExportCmd.Flags().StringArrayVar(&chartExportValues, "values", nil, "Path to values file(s) to use during rendering")
	chartExportCmd.Flags().StringArrayVar(&chartExportSet, "set", nil, "Set individual values (key=value)")
	chartExportCmd.Flags().BoolVar(&chartExportCRDs, "crds", true, "Include CRDs in the output")
	chartExportCmd.Flags().StringVarP(&chartExportNamespace, "namespace", "n", "", "Kubernetes namespace for rendered manifests")
	chartExportCmd.Flags().StringVar(&chartExportRelease, "release", "openfaas", "Helm release name")

	// 将 export 子命令挂载到 chart 命令下
	chartCmd.AddCommand(chartExportCmd)
}

// preRunChartExport 执行前检查：helm 是否存在、chart 目录是否存在
func preRunChartExport(cmd *cobra.Command, args []string) error {
	if _, err := exec.LookPath("helm"); err != nil {
		return fmt.Errorf("helm is required but was not found in PATH")
	}

	chartPath := resolveChartPath(args)
	info, err := os.Stat(chartPath)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("chart directory not found: %s", chartPath)
	}

	return nil
}

// runChartExport 执行 chart 导出主逻辑
func runChartExport(cmd *cobra.Command, args []string) error {
	chartPath := resolveChartPath(args)

	// 构造 helm template 命令参数
	helmArgs := []string{"template", chartExportRelease, chartPath}

	if chartExportCRDs {
		helmArgs = append(helmArgs, "--include-crds")
	}

	for _, vf := range chartExportValues {
		helmArgs = append(helmArgs, "-f", vf)
	}

	for _, s := range chartExportSet {
		helmArgs = append(helmArgs, "--set", s)
	}

	if chartExportNamespace != "" {
		helmArgs = append(helmArgs, "--namespace", chartExportNamespace)
	}

	fmt.Printf("Running: helm %s\n", strings.Join(helmArgs, " "))

	// 执行 helm template
	helmCmd := exec.Command("helm", helmArgs...)
	var stdout, stderr bytes.Buffer
	helmCmd.Stdout = &stdout
	helmCmd.Stderr = &stderr

	if err := helmCmd.Run(); err != nil {
		return fmt.Errorf("helm template failed: %s\n%s", err, stderr.String())
	}

	// 拆分 YAML 流为单个资源
	resources, err := splitYAMLStream(&stdout)
	if err != nil {
		return fmt.Errorf("failed to parse YAML output: %s", err)
	}

	if len(resources) == 0 {
		return fmt.Errorf("no resources found in helm output")
	}

	// 准备输出目录
	outputDir, err := filepath.Abs(chartExportOutput)
	if err != nil {
		return err
	}

	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("failed to clean output directory: %s", err)
	}

	// 检测重复的 kind + name，用于处理重名文件
	type kindName struct{ kind, name string }
	seen := make(map[kindName]int)
	for _, res := range resources {
		seen[kindName{res.Kind, res.Name}]++
	}

	// 写入每个资源到独立文件
	written := 0
	for _, res := range resources {
		dir := strings.ToLower(res.Kind)
		// CRD 文件夹加 00_ 前缀保证排序优先
		if res.Kind == "CustomResourceDefinition" {
			dir = "00_" + dir
		}

		destDir := filepath.Join(outputDir, dir)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %s", destDir, err)
		}

		// 处理重名：加上 namespace
		filename := res.Name
		if seen[kindName{res.Kind, res.Name}] > 1 && res.Namespace != "" {
			filename = res.Name + "." + res.Namespace
		}

		// 写入文件
		destFile := filepath.Join(destDir, filename+".yaml")
		if err := os.WriteFile(destFile, res.Raw, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %s", destFile, err)
		}

		rel, _ := filepath.Rel(outputDir, destFile)
		fmt.Printf("  wrote %s\n", rel)
		written++
	}

	fmt.Printf("\nExported %d resources to %s\n", written, outputDir)
	return nil
}

// chartResource 表示一个 Kubernetes 资源
type chartResource struct {
	Kind      string
	Name      string
	Namespace string
	Raw       []byte
}

// splitYAMLStream 把 helm 输出的 YAML 流按 --- 拆分成多个资源
func splitYAMLStream(r io.Reader) ([]chartResource, error) {
	decoder := yaml.NewDecoder(r)
	var resources []chartResource

	for {
		var doc map[string]interface{}
		err := decoder.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if doc == nil {
			continue
		}

		// 提取资源元信息
		kind, _ := doc["kind"].(string)
		if kind == "" {
			continue
		}

		meta, _ := doc["metadata"].(map[string]interface{})
		if meta == nil {
			continue
		}
		name, _ := meta["name"].(string)
		if name == "" {
			continue
		}
		namespace, _ := meta["namespace"].(string)

		// 重新序列化为干净的 YAML
		raw, err := marshalYAML(doc)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal %s/%s: %s", kind, name, err)
		}

		resources = append(resources, chartResource{
			Kind:      kind,
			Name:      name,
			Namespace: namespace,
			Raw:       raw,
		})
	}

	return resources, nil
}

// marshalYAML 格式化输出 YAML（缩进 2）
func marshalYAML(doc map[string]interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	enc.Close()
	return buf.Bytes(), nil
}

// resolveChartPath 解析 chart 路径：默认 chart/openfaas
func resolveChartPath(args []string) string {
	chartName := "openfaas"
	if len(args) > 0 && args[0] != "" {
		chartName = args[0]
	}
	return filepath.Join("chart", chartName)
}
