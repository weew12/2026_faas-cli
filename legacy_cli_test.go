// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

// translateLegacyOptsTests 测试 translateLegacyOpts 函数的自动化测试套件

package main

import (
	"reflect"
	"testing"
)

// 定义测试用例切片
// 每个用例包含：标题、输入参数、期望输出、是否期望出错
var translateLegacyOptsTests = []struct {
	title        string   // 测试用例名称（描述测试场景）
	inputArgs    []string // 输入：旧版命令行参数
	expectedArgs []string // 期望：转换后的正确新版参数
	expectError  bool     // 是否期望这个用例抛出错误（非法参数时为true）
}{
	{
		title:        "legacy deploy action with all args, no =",
		inputArgs:    []string{"faas-cli", "-action", "deploy", "-image", "testimage", "-name", "fnname", "-fprocess", `"/usr/bin/faas-img2ansi"`, "-gateway", "https://url", "-handler", "/dir/", "-lang", "python", "-replace"},
		expectedArgs: []string{"faas-cli", "deploy", "--image", "testimage", "--name", "fnname", "--fprocess", `"/usr/bin/faas-img2ansi"`, "--gateway", "https://url", "--handler", "/dir/", "--lang", "python", "--replace"},
		expectError:  false,
	},
	{
		title:        "legacy deploy action with =",
		inputArgs:    []string{"faas-cli", "-action=deploy", "-image=testimage", "-name=fnname", `-fprocess="/usr/bin/faas-img2ansi"`},
		expectedArgs: []string{"faas-cli", "deploy", "--image=testimage", "--name=fnname", `--fprocess="/usr/bin/faas-img2ansi"`},
		expectError:  false,
	},
	{
		title:        "legacy deploy action with -f",
		inputArgs:    []string{"faas-cli", "-action=deploy", "-f", "/dir/file.yml"},
		expectedArgs: []string{"faas-cli", "deploy", "-f", "/dir/file.yml"},
		expectError:  false,
	},
	{
		title:        "legacy deploy action with -yaml",
		inputArgs:    []string{"faas-cli", "-action=deploy", "-yaml", "/dir/file.yml"},
		expectedArgs: []string{"faas-cli", "deploy", "--yaml", "/dir/file.yml"},
		expectError:  false,
	},
	{
		title:        "legacy build action with all args, no =",
		inputArgs:    []string{"faas-cli", "-action", "build", "-image", "testimage", "-name", "fnname", "-handler", "/dir/", "-lang", "python", "-no-cache", "-squash"},
		expectedArgs: []string{"faas-cli", "build", "--image", "testimage", "--name", "fnname", "--handler", "/dir/", "--lang", "python", "--no-cache", "--squash"},
		expectError:  false,
	},
	{
		title:        "legacy delete action (note delete->remove translation)",
		inputArgs:    []string{"faas-cli", "-action", "delete", "-name", "fnname"},
		expectedArgs: []string{"faas-cli", "remove", "fnname"}, // delete → remove
		expectError:  false,
	},
	{
		title:        "legacy delete action with yaml",
		inputArgs:    []string{"faas-cli", "-action", "delete", "-f", "/dir/file.yml"},
		expectedArgs: []string{"faas-cli", "remove", "-f", "/dir/file.yml"},
		expectError:  false,
	},
	{
		title:        "legacy version flag",
		inputArgs:    []string{"faas-cli", "-version"},
		expectedArgs: []string{"faas-cli", "version"},
		expectError:  false,
	},
	{
		title:        "version command",
		inputArgs:    []string{"faas-cli", "version"},
		expectedArgs: []string{"faas-cli", "version"},
		expectError:  false,
	},
	{
		title:        "deploy command",
		inputArgs:    []string{"faas-cli", "deploy", "--image", "testimage", "--name", "fnname", "--fprocess", `"/usr/bin/faas-img2ansi"`, "--gateway", "https://url", "--handler", "/dir/", "--lang", "python", "--replace", "--env", "KEY1=VAL1", "--env", "KEY2=VAL2"},
		expectedArgs: []string{"faas-cli", "deploy", "--image", "testimage", "--name", "fnname", "--fprocess", `"/usr/bin/faas-img2ansi"`, "--gateway", "https://url", "--handler", "/dir/", "--lang", "python", "--replace", "--env", "KEY1=VAL1", "--env", "KEY2=VAL2"},
		expectError:  false,
	},
	{
		title:        "build command",
		inputArgs:    []string{"faas-cli", "build", "--image", "testimage", "--name", "fnname", "--handler", "/dir/", "--lang", "python", "--no-cache", "--squash"},
		expectedArgs: []string{"faas-cli", "build", "--image", "testimage", "--name", "fnname", "--handler", "/dir/", "--lang", "python", "--no-cache", "--squash"},
		expectError:  false,
	},
	{
		title:        "remove command",
		inputArgs:    []string{"faas-cli", "remove", "fnname"},
		expectedArgs: []string{"faas-cli", "remove", "fnname"},
		expectError:  false,
	},
	{
		title:        "remove command alias rm",
		inputArgs:    []string{"faas-cli", "rm", "fnname"},
		expectedArgs: []string{"faas-cli", "rm", "fnname"},
		expectError:  false,
	},
	{
		title:        "remove command alias delete",
		inputArgs:    []string{"faas-cli", "delete", "fnname"},
		expectedArgs: []string{"faas-cli", "delete", "fnname"},
		expectError:  false,
	},
	{
		title:        "push command",
		inputArgs:    []string{"faas-cli", "delete", "fnname"},
		expectedArgs: []string{"faas-cli", "delete", "fnname"},
		expectError:  false,
	},
	{
		title:        "bashcompletion command",
		inputArgs:    []string{"faas-cli", "bashcompletion", "/dir/file"},
		expectedArgs: []string{"faas-cli", "bashcompletion", "/dir/file"},
		expectError:  false,
	},
	{
		title:        "legacy flag as value without =",
		inputArgs:    []string{"faas-cli", "-action", "deploy", "-name", `"-name"`},
		expectedArgs: []string{"faas-cli", "deploy", "--name", `"-name"`},
		expectError:  false,
	},
	{
		title:        "legacy flag as value with =",
		inputArgs:    []string{"faas-cli", "-action", "deploy", "-name=-name"},
		expectedArgs: []string{"faas-cli", "deploy", "--name=-name"},
		expectError:  false,
	},
	{
		title:        "unknown legacy flag",
		inputArgs:    []string{"faas-cli", "-action", "deploy", "-fe"},
		expectedArgs: []string{"faas-cli", "deploy", "-fe"},
		expectError:  false,
	},

	// 错误用例（期望报错）
	{
		title:        "legacy -action missing value",
		inputArgs:    []string{"faas-cli", "-action"},
		expectedArgs: []string{""},
		expectError:  true,
	},
	{
		title:        "legacy -action= missing value",
		inputArgs:    []string{"faas-cli", "-action="},
		expectedArgs: []string{""},
		expectError:  true,
	},
	{
		title:        "legacy -action with unknown value",
		inputArgs:    []string{"faas-cli", "-action", "unknownaction"},
		expectedArgs: []string{""},
		expectError:  true,
	},
	{
		title:        "legacy -action= with unknown value",
		inputArgs:    []string{"faas-cli", "-action=unknownaction"},
		expectedArgs: []string{""},
		expectError:  true,
	},
}

// Test_translateLegacyOpts
// 单元测试入口：遍历所有测试用例，执行 translateLegacyOpts 并验证结果
func Test_translateLegacyOpts(t *testing.T) {
	// 遍历每个测试用例
	for _, test := range translateLegacyOptsTests {
		// 每个用例独立运行（t.Run）
		t.Run(test.title, func(t *testing.T) {
			// 调用被测试函数
			actual, err := translateLegacyOpts(test.inputArgs)

			// 如果期望出错，则检查是否真的返回了错误
			if test.expectError {
				if err == nil {
					t.Errorf("TranslateLegacyOpts test [%s] test failed, expected error not thrown", test.title)
					return
				}
			} else {
				// 不期望出错，但却报错了 → 测试失败
				if err != nil {
					t.Errorf("TranslateLegacyOpts test [%s] test failed, unexpected error thrown", test.title)
					return
				}
			}

			// 比较【实际输出】和【期望输出】是否完全一致
			if !reflect.DeepEqual(actual, test.expectedArgs) {
				t.Errorf("TranslateLegacyOpts test [%s] test failed, does not match expected result;\n  actual:   [%v]\n  expected: [%v]",
					test.title,
					actual,
					test.expectedArgs,
				)
			}
		})
	}
}
