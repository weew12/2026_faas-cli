// Copyright (c) OpenFaaS Author(s) 2024. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package commands

import (
	"github.com/spf13/cobra"
)

// init 初始化，将 chart 子命令注册到根命令 faasCmd
func init() {
	faasCmd.AddCommand(chartCmd)
}

// chartCmd Helm chart 相关操作的根命令，用于导出和管理 OpenFaaS Helm charts
var chartCmd = &cobra.Command{
	Use:   `chart`,
	Short: "Helm chart commands",
	Long:  "Export and manage OpenFaaS Helm charts",
}
