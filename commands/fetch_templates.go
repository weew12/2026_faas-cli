// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	execute "github.com/alexellis/go-execute/v2"
	"github.com/openfaas/faas-cli/builder"
	"github.com/openfaas/faas-cli/versioncontrol"
)

// DefaultTemplateRepository 官方模板仓库地址
const DefaultTemplateRepository = "https://github.com/openfaas/templates.git"

// TemplateDirectory 本地存放函数模板的目录
const TemplateDirectory = "./template/"

// ShaPrefix Git 提交哈希的前缀
const ShaPrefix = "sha-"

// fetchTemplates 使用 git clone 拉取代码模板
func fetchTemplates(templateURL, refName, templateName string, overwriteTemplates bool) error {
	if len(templateURL) == 0 {
		return fmt.Errorf("pass valid templateURL")
	}

	refMsg := ""
	if len(refName) > 0 {
		refMsg = " [" + refName + "]"
	}

	log.Printf("Fetching templates from %s%s", templateURL, refMsg)

	// 创建临时目录存放克隆的模板
	extractedPath, err := os.MkdirTemp("", "openfaas-templates-*")
	if err != nil {
		return fmt.Errorf("unable to create temporary directory: %s", err)
	}

	// 调试模式不删除临时文件
	if !pullDebug {
		defer os.RemoveAll(extractedPath)
	}

	pullDebugPrint(fmt.Sprintf("Temp files in %s", extractedPath))

	args := map[string]string{"dir": extractedPath, "repo": templateURL}
	cmd := versioncontrol.GitCloneDefault

	// 如果指定了分支/标签/提交哈希
	if len(refName) > 0 {
		if strings.HasPrefix(refName, ShaPrefix) {
			// 完整克隆（用于检出指定 commit）
			cmd = versioncontrol.GitCloneFullDepth
		} else {
			// 浅克隆指定分支
			cmd = versioncontrol.GitCloneBranch
			args["refname"] = refName
		}
	}

	// 执行 git clone
	if err := cmd.Invoke(".", args); err != nil {
		return fmt.Errorf("error invoking git clone: %w", err)
	}

	// 如果是指定 commit SHA，需要额外 checkout
	if len(refName) > 0 && strings.HasPrefix(refName, ShaPrefix) {

		targetCommit := strings.TrimPrefix(refName, ShaPrefix)

		// 校验 SHA 格式
		if !regexp.MustCompile(`^[a-fA-F0-9]{7,40}$`).MatchString(targetCommit) {
			return fmt.Errorf("invalid SHA format: %s - must be 7-40 hex characters", targetCommit)
		}

		// 检出指定 commit
		t := execute.ExecTask{
			Command: "git",
			Args:    []string{"-C", extractedPath, "checkout", targetCommit},
		}
		res, err := t.Execute(context.Background())
		if err != nil {
			return fmt.Errorf("error checking out ref %s: %w", targetCommit, err)
		}
		if res.ExitCode != 0 {
			out := res.Stdout + " " + res.Stderr
			return fmt.Errorf("error checking out ref %s: %s", targetCommit, out)
		}
	}

	// 调试：打印最后一次提交信息
	if os.Getenv("FAAS_DEBUG") == "1" {
		task := execute.ExecTask{
			Command: "git",
			Args:    []string{"-C", extractedPath, "log", "-1", "--oneline"},
		}

		res, err := task.Execute(context.Background())
		if err != nil {
			return fmt.Errorf("error executing git log: %w", err)
		}
		if res.ExitCode != 0 {
			e := fmt.Errorf("exit code: %d, stderr: %s, stdout: %s", res.ExitCode, res.Stderr, res.Stdout)
			return fmt.Errorf("error from: git log: %w", e)
		}

		log.Printf("[git] log: %s", strings.TrimSpace(res.Stdout))
	}

	// 获取克隆后的完整 SHA
	sha, err := versioncontrol.GetGitSHAFor(extractedPath, false)
	if err != nil {
		return err
	}

	// 获取当前目录，创建本地 template 目录
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("can't get current working directory: %s", err)
	}
	localTemplatesDir := filepath.Join(cwd, TemplateDirectory)
	if _, err := os.Stat(localTemplatesDir); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(localTemplatesDir, 0755); err != nil && !os.IsExist(err) {
			return fmt.Errorf("error creating template directory: %s - %w", localTemplatesDir, err)
		}
	}

	// 将模板从临时目录移动到本地 template 目录
	protectedLanguages, fetchedLanguages, err := moveTemplates(localTemplatesDir, extractedPath, templateName, overwriteTemplates, templateURL, refName, sha)
	if err != nil {
		return err
	}

	// 如果有受保护（已存在且不覆盖）的模板，返回错误
	if len(protectedLanguages) > 0 {
		return fmt.Errorf("unable to overwrite the following: %v", protectedLanguages)
	}

	fmt.Printf("Wrote %d template(s) : %v\n", len(fetchedLanguages), fetchedLanguages)

	return err
}

// canWriteLanguage 判断是否可以写入该语言模板
// 允许覆盖 或 本地不存在该模板时返回 true
func canWriteLanguage(existingLanguages []string, language string, overwriteTemplate bool) bool {
	if overwriteTemplate {
		return true
	}

	return !slices.Contains(existingLanguages, language)
}

// moveTemplates 将克隆的模板移动到本地模板目录
// 返回：受保护的模板、成功拉取的模板、错误
func moveTemplates(localTemplatesDir, extractedPath, templateName string, overwriteTemplate bool, repository string, refName string, sha string) ([]string, []string, error) {
	// 去掉 @ref 后缀
	template := strings.SplitN(templateName, "@", 2)[0]

	var (
		existingLanguages  []string // 本地已存在的模板
		fetchedLanguages   []string // 本次拉取到的模板
		protectedLanguages []string // 无法覆盖的受保护模板
		err                error
	)

	// 读取本地已存在的模板
	templateEntries, err := os.ReadDir(localTemplatesDir)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to read directory: %s", localTemplatesDir)
	}

	for _, entry := range templateEntries {
		if !entry.IsDir() {
			continue
		}

		// 检查是否有有效的 template.yml
		templateFile := filepath.Join(localTemplatesDir, entry.Name(), "template.yml")
		if _, err := os.Stat(templateFile); err != nil && !os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("can't find template.yml in: %s", templateFile)
		}

		existingLanguages = append(existingLanguages, entry.Name())
	}

	// 读取临时目录中的模板
	extractedTemplates, err := os.ReadDir(filepath.Join(extractedPath, TemplateDirectory))
	if err != nil {
		return nil, nil, fmt.Errorf("can't find templates in: %s", filepath.Join(extractedPath, TemplateDirectory))
	}

	// 遍历并复制模板
	for _, entry := range extractedTemplates {
		if !entry.IsDir() {
			continue
		}
		language := entry.Name()
		refSuffix := ""
		if refName != "" {
			refSuffix = "@" + refName
		}

		// 如果指定了模板名称，只复制对应模板
		if len(template) > 0 && language != template {
			continue
		}

		// 判断是否可写入
		if canWriteLanguage(existingLanguages, language, overwriteTemplate) {
			languageSrc := filepath.Join(extractedPath, TemplateDirectory, language)

			var languageDest string

			// 目标路径：支持自定义名称
			if len(templateName) > 0 {
				languageDest = filepath.Join(localTemplatesDir, templateName)
			} else {
				languageDest = filepath.Join(localTemplatesDir, language)
				if refName != "" {
					languageDest += "@" + refName
				}
			}

			// 记录拉取的模板名
			langName := language
			if refName != "" {
				langName = language + "@" + refName
			}
			fetchedLanguages = append(fetchedLanguages, langName)

			// 复制文件
			if err := builder.CopyFiles(languageSrc, languageDest); err != nil {
				return nil, nil, err
			}

			// 写入元数据文件
			if err := writeTemplateMeta(languageDest, repository, refName, sha); err != nil {
				return nil, nil, err
			}
		} else {
			// 已存在且不允许覆盖，加入保护列表
			protectedLanguages = append(protectedLanguages, language+refSuffix)
			continue
		}
	}

	return protectedLanguages, fetchedLanguages, nil
}

// writeTemplateMeta 为模板写入元信息（仓库、引用、SHA、时间）
func writeTemplateMeta(languageDest, repository, refName, sha string) error {
	templateMeta := TemplateMeta{
		Repository: repository,
		WrittenAt:  time.Now(),
		RefName:    refName,
		Sha:        sha,
	}

	metaBytes, err := json.Marshal(templateMeta)
	if err != nil {
		return fmt.Errorf("error marshalling template meta: %s", err)
	}

	metaPath := filepath.Join(languageDest, "meta.json")
	if err := os.WriteFile(metaPath, metaBytes, 0644); err != nil {
		return fmt.Errorf("error writing template meta: %s", err)
	}

	return nil
}

// pullTemplate 拉取模板的入口方法，解析仓库、引用、执行拉取
func pullTemplate(repository, templateName string, overwriteTemplates bool) error {

	baseRepository := repository

	// 如果是本地路径，处理 #ref 格式
	if _, err := os.Stat(repository); err != nil && os.IsNotExist(err) {
		base, _, found := strings.Cut(repository, "#")
		if found {
			baseRepository = base
		} else {
			_, ref, found := strings.Cut(templateName, "#")
			if found {
				repository = baseRepository + "#" + ref
			}
		}
	}

	// 验证是否为合法的 Git 仓库
	if !isValidFilesystemPath(repository) {
		if !versioncontrol.IsGitRemote(baseRepository) && !versioncontrol.IsPinnedGitRemote(baseRepository) {
			return fmt.Errorf("the repository URL must be a valid git repo uri")
		}
	}

	// 解析出仓库地址和引用（branch/tag/sha）
	repository, refName := versioncontrol.ParsePinnedRemote(repository)
	isShaRefName := strings.HasPrefix(refName, ShaPrefix)

	// 验证引用名格式
	if refName != "" && !isShaRefName {
		err := versioncontrol.GitCheckRefName.Invoke("", map[string]string{"refname": refName})
		if err != nil {
			fmt.Printf("Invalid tag or branch name `%s`\n", refName)
			fmt.Println("See https://git-scm.com/docs/git-check-ref-format for more details of the rules Git enforces on branch and reference names.")

			return err
		}
	}

	// 执行拉取
	if err := fetchTemplates(repository, refName, templateName, overwriteTemplates); err != nil {
		return fmt.Errorf("error while fetching templates: %w", err)
	}

	return nil
}

// TemplateMeta 模板元数据结构，保存在 meta.json
type TemplateMeta struct {
	Repository string    `json:"repository"`
	RefName    string    `json:"ref_name,omitempty"`
	Sha        string    `json:"sha,omitempty"`
	WrittenAt  time.Time `json:"written_at"`
}

// isValidFilesystemPath 判断是否为有效本地路径
func isValidFilesystemPath(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}
