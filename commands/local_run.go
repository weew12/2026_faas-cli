package commands

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"os/exec"
	"os/signal"

	"github.com/openfaas/faas-cli/builder"
	"github.com/openfaas/faas-cli/schema"
	"github.com/openfaas/go-sdk/stack"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// localSecretsDir 本地测试时使用的密钥目录
const localSecretsDir = ".secrets"

func init() {
	// 注册 local-run 命令到主命令
	faasCmd.AddCommand(newLocalRunCmd())
}

// runOptions 本地运行命令的配置选项
type runOptions struct {
	print    bool              // 仅打印 docker 命令，不执行
	port     int               // 映射的主机端口
	network  string            // 使用的 Docker 网络
	extraEnv map[string]string // 额外的环境变量
	output   io.Writer         // 标准输出
	err      io.Writer         // 错误输出
	build    bool              // 运行前是否自动构建
}

// opts 全局命令选项
var opts runOptions

// newLocalRunCmd 创建 local-run 命令
func newLocalRunCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:   `local-run NAME --port PORT -f YAML_FILE [flags from build]`,
		Short: "Start a function with docker for local testing (experimental feature)",
		Long: `Providing faas-cli build has already been run, this command will use the 
docker command to start a container on your local machine using its image.

The function will be bound to the port specified by the --port flag, or 8080
by default.

There is limited support for secrets, and the function cannot contact other 
services deployed within your OpenFaaS cluster.`,
		Example: `
  # Run a function locally
  faas-cli local-run stronghash

  # Run on a custom port
  faas-cli local-run stronghash --port 8081

  # Run on a random port
  faas-cli local-run -p 0

  # Use a custom YAML file other than stack.yaml
  faas-cli local-run stronghash -f ./stronghash.yaml
`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// 只允许传入一个函数名
			if len(args) > 1 {
				return fmt.Errorf("only one function name is allowed")
			}
			_, err := cmd.Flags().GetBool("watch")
			if err != nil {
				return err
			}

			return nil
		},
		RunE: runLocalRunE,
	}

	// 注册命令行参数
	cmd.Flags().BoolVar(&opts.print, "print", false, "Print the docker command instead of running it")
	cmd.Flags().BoolVar(&opts.build, "build", true, "Build function prior to local-run")
	cmd.Flags().IntVarP(&opts.port, "port", "p", 8080, "port to bind the function to, set to \"0\" to use a random port")
	cmd.Flags().Var(&tagFormat, "tag", "Override latest tag on function Docker image, accepts 'digest', 'sha', 'branch', 'describe', or 'latest'")

	cmd.Flags().StringVar(&opts.network, "network", "", "connect function to an existing network, use 'host' to access other process already running on localhost. When using this, '--port' is ignored, if you have port collisions, you may change the port using '-e port=NEW_PORT'")
	cmd.Flags().StringToStringVarP(&opts.extraEnv, "env", "e", map[string]string{}, "additional environment variables (ENVVAR=VALUE), use this to experiment with different values for your function")
	cmd.Flags().BoolVar(&watch, "watch", false, "Watch for changes in files and re-deploy")

	// 继承 build 命令的所有参数
	build, _, _ := faasCmd.Find([]string{"build"})
	cmd.Flags().AddFlagSet(build.Flags())

	return cmd
}

// runLocalRunE 命令入口：处理监听模式/直接运行
func runLocalRunE(cmd *cobra.Command, args []string) error {

	watch, _ := cmd.Flags().GetBool("watch")

	// 监听模式（暂未完全实现）
	if watch {
		return watchLoop(cmd, args, localRunExec)
	}

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	return localRunExec(cmd, args, ctx)
}

// localRunExec 执行本地运行逻辑：构建 → 运行
func localRunExec(cmd *cobra.Command, args []string, ctx context.Context) error {
	if opts.build {
		if err := localBuild(cmd, args); err != nil {
			return err
		}
	}

	opts.output = cmd.OutOrStdout()
	opts.err = cmd.ErrOrStderr()

	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	return runFunction(ctx, name, opts)

}

// localBuild 运行前自动构建函数
func localBuild(cmd *cobra.Command, args []string) error {
	if err := preRunBuild(cmd, args); err != nil {
		return err
	}

	if len(args) > 0 {
		fmt.Println("Building: " + args[0])
		if args[0] != "" {
			filter = args[0]
		}
	}

	if err := runBuild(cmd, args); err != nil {
		return err
	}

	return nil
}

// runFunction 核心：启动 Docker 容器运行函数
func runFunction(ctx context.Context, name string, opts runOptions) error {
	var services *stack.Services

	// 未指定函数名 → 读取 yaml 并检查函数数量
	if len(name) == 0 {
		s, err := stack.ParseYAMLFile(yamlFile, "", "", true)
		if err != nil {
			return err
		}

		if err = updateGitignore(); err != nil {
			return err
		}

		services = s

		if len(services.Functions) == 0 {
			return fmt.Errorf("no functions found in the stack file")
		}

		if len(services.Functions) > 1 {
			fnList := []string{}
			for key := range services.Functions {
				fnList = append(fnList, key)
			}
			return fmt.Errorf("give a function name to run: %v", fnList)
		}

		for key := range services.Functions {
			name = key
			break
		}
	} else {
		// 指定了函数名 → 只解析该函数
		s, err := stack.ParseYAMLFile(yamlFile, "", name, true)
		if err != nil {
			return err
		}
		services = s

		if len(services.Functions) == 0 {
			return fmt.Errorf("no functions matching %q in the stack file", name)
		}
	}

	// 先清理旧容器
	removeContainer(name)

	function := services.Functions[name]

	functionNamespace = function.Namespace
	if len(functionNamespace) == 0 {
		functionNamespace = "openfaas-fn"
	}

	// 设置 OpenFaaS 标准环境变量
	opts.extraEnv["OPENFAAS_NAME"] = name
	opts.extraEnv["OPENFAAS_NAMESPACE"] = functionNamespace

	// 默认开启本地 JWT 认证
	opts.extraEnv["jwt_auth_local"] = "true"

	// 端口为 0 时自动分配随机端口
	if opts.port == 0 {
		randomPort, err := getPort()
		if err != nil {
			return err
		}
		opts.port = randomPort
	}

	// 构建完整的 docker run 命令
	cmd, err := buildDockerRun(ctx, name, function, opts)
	if err != nil {
		return err
	}

	// 仅打印命令模式
	if opts.print {
		fmt.Fprintf(opts.output, "%s\n", cmd.String())
		return nil
	}

	// 监听退出信号
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	cmd.Stdout = opts.output
	cmd.Stderr = opts.err

	fmt.Printf("Starting local-run for: %s on: http://0.0.0.0:%d\n\n", name, opts.port)
	grpContext := context.Background()
	grpContext, cancel := context.WithCancel(grpContext)
	defer cancel()

	errGrp, _ := errgroup.WithContext(grpContext)

	// 协程1：启动容器
	errGrp.Go(func() error {
		if err = cmd.Start(); err != nil {
			return err
		}

		if err := cmd.Wait(); err != nil {
			if strings.Contains(err.Error(), "signal: killed") {
				return nil
			} else if strings.Contains(err.Error(), "os: process already finished") {
				return nil
			}

			return err
		}
		return nil
	})

	// 退出时清理容器
	defer func() {
		removeContainer(name)
	}()

	// 协程2：监听信号退出
	errGrp.Go(func() error {

		select {
		case <-sigs:
			log.Printf("Caught signal, exiting")
			cancel()
		case <-ctx.Done():
			log.Printf("Context cancelled, exiting..")
			cancel()
		}
		return nil
	})

	return errGrp.Wait()
}

// getPort 获取一个随机可用端口
func getPort() (int, error) {

	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}

	defer l.Close()

	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return 0, err
	}

	if port != "" {
		return strconv.Atoi(port)
	}

	return 0, fmt.Errorf("unable to get a port")
}

// removeContainer 强制删除同名容器
func removeContainer(name string) {
	runDockerRm := exec.Command("docker", "rm", "-f", name)
	runDockerRm.Run()
}

// buildDockerRun 构造完整的 docker run 命令
func buildDockerRun(ctx context.Context, name string, fnc stack.Function, opts runOptions) (*exec.Cmd, error) {
	args := []string{"run", "--name", name, "--rm", "-i", fmt.Sprintf("-p=%d:8080", opts.port)}

	// 指定网络
	if opts.network != "" {
		args = append(args, fmt.Sprintf("--network=%s", opts.network))
	}

	// 获取 fprocess 启动命令
	fprocess, err := deriveFprocess(fnc)
	if err != nil {
		return nil, err
	}

	// 注入环境变量
	for name, value := range fnc.Environment {
		args = append(args, fmt.Sprintf("-e=%s=%s", name, value))
	}

	// 读取环境文件
	moreEnv, err := readFiles(fnc.EnvironmentFile)
	if err != nil {
		return nil, err
	}

	for name, value := range moreEnv {
		args = append(args, fmt.Sprintf("-e=%s=%s", name, value))
	}

	// 额外环境变量
	for name, value := range opts.extraEnv {
		args = append(args, fmt.Sprintf("-e=%s=%s", name, value))
	}

	// 只读根文件系统
	if fnc.ReadOnlyRootFilesystem {
		args = append(args, "--read-only")
	}

	// 资源限制
	if fnc.Limits != nil {
		if fnc.Limits.Memory != "" {
			args = append(args, fmt.Sprintf("--memory-reservation=%s", fnc.Limits.Memory))
		}

		if fnc.Limits.CPU != "" {
			args = append(args, fmt.Sprintf("--cpus=%s", fnc.Limits.CPU))
		}
	}

	// 处理本地密钥（.secrets 目录）
	if len(fnc.Secrets) > 0 {
		secretsPath, err := filepath.Abs(localSecretsDir)
		if err != nil {
			return nil, fmt.Errorf("can't determine secrets folder: %w", err)
		}

		if err = os.MkdirAll(secretsPath, 0700); err != nil && !os.IsExist(err) {
			return nil, fmt.Errorf("error creating local secrets folder %q: %w", secretsPath, err)
		}

		if !opts.print {
			err = dirContainsFiles(secretsPath, fnc.Secrets...)
			if err != nil {
				return nil, fmt.Errorf("missing files: %w", err)
			}
		}

		args = append(args, fmt.Sprintf("--volume=%s:/var/openfaas/secrets", secretsPath))
	}

	// 设置 fprocess
	if fprocess != "" {
		args = append(args, fmt.Sprintf("-e=fprocess=%s", fprocess))
	}

	// 构建镜像名称
	branch, version, err := builder.GetImageTagValues(tagFormat, fnc.Handler)
	if err != nil {
		return nil, err
	}

	imageName := schema.BuildImageName(tagFormat, fnc.Image, version, branch)

	fmt.Printf("Image: %s\n", imageName)

	args = append(args, imageName)
	cmd := exec.CommandContext(ctx, "docker", args...)

	return cmd, nil
}

// dirContainsFiles 检查密钥文件是否存在
func dirContainsFiles(dir string, names ...string) error {
	var err = &missingFileError{
		dir:     dir,
		missing: []string{},
	}

	for _, name := range names {
		path := filepath.Join(dir, name)
		_, statErr := os.Stat(path)
		if statErr != nil {
			err.missing = append(err.missing, name)
		}
	}

	if len(err.missing) > 0 {
		return err
	}

	return nil
}

// missingFileError 密钥文件缺失错误
type missingFileError struct {
	missing []string
	dir     string
}

func (m missingFileError) Error() string {
	return fmt.Sprintf("create the following secrets (%s) in: %q", strings.Join(m.missing, ", "), m.dir)
}

func (m *missingFileError) AddMissingSecret(p string) {
	m.missing = append(m.missing, p)
}
