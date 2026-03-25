// Copyright (c) OpenFaaS Author(s) 2018. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"bytes"
	"fmt"
	"os"
	"sort"

	"github.com/openfaas/faas-cli/builder"
	v2 "github.com/openfaas/faas-cli/schema/store/v2"

	"github.com/openfaas/faas-cli/proxy"
	"github.com/openfaas/faas-cli/schema"
	knativev1 "github.com/openfaas/faas-cli/schema/knative/v1"
	openfaasv1 "github.com/openfaas/faas-cli/schema/openfaas/v1"
	"github.com/openfaas/faas-cli/util"
	"github.com/openfaas/go-sdk/stack"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	yaml "gopkg.in/yaml.v3"
)

// 常量定义：CRD 资源类型和默认 API 版本
const (
	resourceKind      = "Function"
	defaultAPIVersion = "openfaas.com/v1"
)

// 全局命令行参数
var (
	api                  string   // CRD API 版本
	name                 string   // 函数名（从商店生成时使用）
	functionNamespace    string   // 函数命名空间
	crdFunctionNamespace string   // CRD 使用的命名空间
	fromStore            string   // 从函数商店生成
	desiredArch          string   // 目标架构
	annotationArgs       []string // 注解参数
	labelArgs            []string // 标签参数
)

func init() {
	// 注册命令行标志
	generateCmd.Flags().StringVar(&fromStore, "from-store", "", "generate using a store image")
	generateCmd.Flags().StringVar(&name, "name", "", "for use with --from-store, override the name for the Function CR")

	generateCmd.Flags().StringVar(&api, "api", defaultAPIVersion, "CRD API version e.g openfaas.com/v1, serving.knative.dev/v1")
	generateCmd.Flags().StringVarP(&crdFunctionNamespace, "namespace", "n", "openfaas-fn", "Kubernetes namespace for functions")
	generateCmd.Flags().Var(&tagFormat, "tag", "Override latest tag on function Docker image, accepts 'digest', 'latest', 'sha', 'branch', 'describe'")
	generateCmd.Flags().BoolVar(&envsubst, "envsubst", true, "Substitute environment variables in stack.yaml file")
	generateCmd.Flags().StringVar(&desiredArch, "arch", "x86_64", "Desired image arch. (Default x86_64)")
	generateCmd.Flags().StringArrayVar(&annotationArgs, "annotation", []string{}, "Any annotations you want to add (to store functions only)")
	generateCmd.Flags().StringArrayVar(&labelArgs, "label", []string{}, "Any labels you want to add (to store functions only)")

	// 添加到主命令
	faasCmd.AddCommand(generateCmd)
}

// generateCmd 生成 Kubernetes CRD YAML 文件
var generateCmd = &cobra.Command{
	Use:   "generate --api=openfaas.com/v1 --yaml stack.yaml --tag sha --namespace=openfaas-fn",
	Short: "Generate Kubernetes CRD YAML file",
	Long:  `The generate command creates kubernetes CRD YAML file for functions`,
	Example: `faas-cli generate --api=openfaas.com/v1 --yaml stack.yaml | kubectl apply  -f -
faas-cli generate --api=openfaas.com/v1 -f stack.yaml
faas-cli generate --api=serving.knative.dev/v1 -f stack.yaml
faas-cli generate --api=openfaas.com/v1 --namespace openfaas-fn -f stack.yaml
faas-cli generate --api=openfaas.com/v1 -f stack.yaml --tag branch -n openfaas-fn`,
	PreRunE: preRunGenerate,
	RunE:    runGenerate,
}

// preRunGenerate 执行前校验：必须提供 API 版本
func preRunGenerate(cmd *cobra.Command, args []string) error {
	if len(api) == 0 {
		return fmt.Errorf("you must supply the API version with the --api flag")
	}

	return nil
}

// filterStoreItem 从商店函数列表中查找指定函数
func filterStoreItem(items []v2.StoreFunction, fromStore string) (*v2.StoreFunction, error) {
	var item *v2.StoreFunction

	for _, val := range items {
		if val.Name == fromStore {
			item = &val
			break
		}
	}

	if item == nil {
		return nil, fmt.Errorf("unable to find '%s' in store", fromStore)
	}

	return item, nil
}

// runGenerate 命令执行主逻辑
func runGenerate(cmd *cobra.Command, args []string) error {

	desiredArch, _ := cmd.Flags().GetString("arch")
	var services stack.Services

	// 解析注解和标签
	annotations, annotationErr := util.ParseMap(annotationArgs, "annotation")
	if annotationErr != nil {
		return fmt.Errorf("error parsing annotations: %v", annotationErr)
	}

	labels, err := util.ParseMap(labelArgs, "label")
	if err != nil {
		return fmt.Errorf("error parsing labels: %v", err)
	}

	// 情况 1：从函数商店生成
	if fromStore != "" {
		if tagFormat == schema.DigestFormat {
			return fmt.Errorf("digest tag format is not supported for store functions")
		}

		// 初始化服务结构
		services = stack.Services{
			Provider: stack.Provider{
				Name: "openfaas",
			},
			Version: "1.0",
		}

		services.Functions = make(map[string]stack.Function)

		// 获取函数商店列表
		items, err := proxy.FunctionStoreList(storeAddress)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("Unable to retrieve functions from URL %s", storeAddress))
		}

		// 查找指定函数
		item, err := filterStoreItem(items, fromStore)
		if err != nil {
			return err
		}

		// 检查架构是否支持
		_, ok := item.Images[desiredArch]
		if !ok {
			var keys []string
			for k := range item.Images {
				keys = append(keys, k)
			}
			return errors.New(fmt.Sprintf("image for %s not found in store. \noptions: %s", desiredArch, keys))
		}

		// 合并注解和标签
		allAnnotations := util.MergeMap(item.Annotations, annotations)

		// 设置 fprocess
		if len(item.Fprocess) > 0 {
			if item.Environment == nil {
				item.Environment = make(map[string]string)
			}

			if _, ok := item.Environment["fprocess"]; !ok {
				item.Environment["fprocess"] = item.Fprocess
			}
		}

		allLabels := util.MergeMap(item.Labels, labels)

		// 函数名（可覆盖）
		fullName := item.Name
		if len(name) > 0 {
			fullName = name
		}

		// 构造函数配置
		services.Functions[fullName] = stack.Function{
			Name:        fullName,
			Image:       item.Images[desiredArch],
			Labels:      &allLabels,
			Annotations: &allAnnotations,
			Environment: item.Environment,
			FProcess:    item.Fprocess,
		}

		// 情况 2：从本地 stack.yaml 生成
	} else if len(yamlFile) > 0 {
		parsedServices, err := stack.ParseYAMLFile(yamlFile, regex, filter, envsubst)
		if err != nil {
			return err
		}

		if parsedServices != nil {
			services = *parsedServices
		}
	} else {
		// 没有提供任何来源
		fmt.Println(
			`No "stack.yaml" or "stack.yml" file was found in the current directory.
Use "--yaml" / "-f" to specify a custom filename.

Alternatively, to generate a definition for store functions, use "--from-store"`)
		os.Exit(1)
	}

	// 生成 CRD YAML
	objectsString, err := generateCRDYAML(services, tagFormat, api, crdFunctionNamespace,
		builder.NewFunctionMetadataSourceLive())
	if err != nil {
		return err
	}

	// 输出结果
	if len(objectsString) > 0 {
		fmt.Println(objectsString)
	}
	return nil
}

// generateCRDYAML 根据函数配置生成 OpenFaaS CRD YAML
func generateCRDYAML(services stack.Services, format schema.BuildFormat, apiVersion, namespace string, metadataSource builder.FunctionMetadataSource) (string, error) {

	var objectsString string

	if len(services.Functions) > 0 {

		// 如果是 Knative API，则单独处理
		if apiVersion == knativev1.APIVersionLatest {
			return generateknativev1ServingServiceCRDYAML(services, format, api, crdFunctionNamespace)
		}

		// 按名称排序函数
		orderedNames := generateFunctionOrder(services.Functions)

		// 遍历每个函数生成 CRD
		for _, name := range orderedNames {

			function := services.Functions[name]
			// 从环境文件读取变量
			fileEnvironment, err := readFiles(function.EnvironmentFile)
			if err != nil {
				return "", err
			}

			// 合并所有环境变量
			allEnvironment, envErr := compileEnvironment([]string{}, function.Environment, fileEnvironment)
			if envErr != nil {
				return "", envErr
			}

			// 获取镜像标签信息
			branch, version, err := metadataSource.Get(tagFormat, function.Handler)
			if err != nil {
				return "", err
			}

			// 构造元数据和镜像名
			metadata := schema.Metadata{Name: name, Namespace: namespace}
			imageName := schema.BuildImageName(format, function.Image, version, branch)

			// 构造 OpenFaaS CRD 结构
			spec := openfaasv1.Spec{
				Name:                   name,
				Image:                  imageName,
				Environment:            allEnvironment,
				Labels:                 function.Labels,
				Annotations:            function.Annotations,
				Limits:                 function.Limits,
				Requests:               function.Requests,
				Constraints:            function.Constraints,
				Secrets:                function.Secrets,
				ReadOnlyRootFilesystem: function.ReadOnlyRootFilesystem,
			}

			crd := openfaasv1.CRD{
				APIVersion: apiVersion,
				Kind:       resourceKind,
				Metadata:   metadata,
				Spec:       spec,
			}

			// YAML 编码
			var buff bytes.Buffer
			yamlEncoder := yaml.NewEncoder(&buff)
			yamlEncoder.SetIndent(2)
			if err := yamlEncoder.Encode(&crd); err != nil {
				return "", err
			}

			objectString := buff.String()
			objectsString += "---\n" + string(objectString)
		}
	}

	return objectsString, nil
}

// generateknativev1ServingServiceCRDYAML 生成 Knative Serving 格式的 CRD
func generateknativev1ServingServiceCRDYAML(services stack.Services, format schema.BuildFormat, apiVersion, namespace string) (string, error) {
	crds := []knativev1.ServingServiceCRD{}

	orderedNames := generateFunctionOrder(services.Functions)

	for _, name := range orderedNames {

		function := services.Functions[name]

		// 读取环境文件
		fileEnvironment, err := readFiles(function.EnvironmentFile)
		if err != nil {
			return "", err
		}

		// 合并环境变量
		allEnvironment, envErr := compileEnvironment([]string{}, function.Environment, fileEnvironment)
		if envErr != nil {
			return "", envErr
		}

		env := orderknativeEnv(allEnvironment)

		var annotations map[string]string
		if function.Annotations != nil {
			annotations = *function.Annotations
		}

		// 获取镜像信息
		branch, version, err := builder.GetImageTagValues(tagFormat, function.Handler)
		if err != nil {
			return "", err
		}

		imageName := schema.BuildImageName(format, function.Image, version, branch)

		// 构造 Knative CRD
		crd := knativev1.ServingServiceCRD{
			Metadata: schema.Metadata{
				Name:        name,
				Namespace:   namespace,
				Annotations: annotations,
			},
			APIVersion: apiVersion,
			Kind:       "Service",

			Spec: knativev1.ServingServiceSpec{
				ServingServiceSpecTemplate: knativev1.ServingServiceSpecTemplate{
					Template: knativev1.ServingServiceSpecTemplateSpec{
						Containers: []knativev1.ServingSpecContainersContainerSpec{},
					},
				},
			},
		}

		crd.Spec.Template.Containers = append(crd.Spec.Template.Containers, knativev1.ServingSpecContainersContainerSpec{
			Image: imageName,
			Env:   env,
		})

		// 处理密钥挂载
		var mounts []knativev1.VolumeMount
		var volumes []knativev1.Volume

		for _, secret := range function.Secrets {
			mounts = append(mounts, knativev1.VolumeMount{
				MountPath: "/var/openfaas/secrets/" + secret,
				ReadOnly:  true,
				Name:      secret,
			})
			volumes = append(volumes, knativev1.Volume{
				Name: secret,
				Secret: knativev1.Secret{
					SecretName: secret,
				},
			})
		}

		crd.Spec.Template.Volumes = volumes
		crd.Spec.Template.Containers[0].VolumeMounts = mounts

		crds = append(crds, crd)
	}

	// 编码输出
	var objectsString string
	for _, crd := range crds {

		var buff bytes.Buffer
		yamlEncoder := yaml.NewEncoder(&buff)
		yamlEncoder.SetIndent(2)
		if err := yamlEncoder.Encode(&crd); err != nil {
			return "", err
		}

		objectsString += "---\n" + string(buff.Bytes())
	}

	return objectsString, nil
}

// generateFunctionOrder 对函数名进行排序
func generateFunctionOrder(functions map[string]stack.Function) []string {

	var functionNames []string

	for functionName := range functions {
		functionNames = append(functionNames, functionName)
	}

	sort.Strings(functionNames)

	return functionNames
}

// orderknativeEnv 对环境变量按字母排序（Knative 要求）
func orderknativeEnv(environment map[string]string) []knativev1.EnvPair {

	var orderedEnvironment []string
	var envVars []knativev1.EnvPair

	for k := range environment {
		orderedEnvironment = append(orderedEnvironment, k)
	}

	sort.Strings(orderedEnvironment)

	for _, envVar := range orderedEnvironment {
		envVars = append(envVars, knativev1.EnvPair{Name: envVar, Value: environment[envVar]})
	}

	return envVars
}
