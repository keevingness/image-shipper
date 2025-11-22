package pull

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/keevingness/image-shipper/internal/config"
	"github.com/keevingness/image-shipper/pkg/docker"
)

// Run 执行pull命令
func Run() {
	// 检查参数
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// 检查是否请求帮助
	if len(os.Args) == 2 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		printUsage()
		return
	}

	if len(os.Args) < 3 {
		printUsage()
		os.Exit(1)
	}

	imageName := os.Args[2]
	if imageName == "" {
		fmt.Println("错误: 镜像名称不能为空")
		os.Exit(1)
	}

	// 加载配置
	cfg, err := config.LoadWithDefaults()
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 检查是否使用podman或自定义容器运行时
	containerRuntime := cfg.Pull.ContainerRuntime // 从配置文件获取默认值

	// 检查命令行参数是否覆盖了容器运行时
	if len(os.Args) > 3 {
		for i := 3; i < len(os.Args); i++ {
			if os.Args[i] == "--podman" {
				containerRuntime = "podman"
			} else if os.Args[i] == "--docker" {
				containerRuntime = "docker"
			} else if os.Args[i] == "-e" && i+1 < len(os.Args) {
				// 使用自定义容器运行时
				containerRuntime = os.Args[i+1]
				break
			}
		}
	}

	// 从配置中获取源镜像仓库地址
	sourceRegistry := cfg.Pull.SourceRegistry

	// 构建完整的源镜像地址
	sourceImage := sourceRegistry + "/" + imageName

	// 解析目标镜像地址，确保格式正确
	_, _, tag, err := docker.ParseImageReference(imageName)
	if err != nil {
		fmt.Printf("错误: 无效的镜像地址格式: %v\n", err)
		os.Exit(1)
	}

	// 如果没有指定标签，默认使用latest
	if tag == "" {
		imageName = imageName + ":latest"
		sourceImage = sourceImage + ":latest"
	}

	// 拉取源镜像
	fmt.Printf("正在从 %s 拉取镜像 %s (使用 %s)...\n", sourceRegistry, imageName, containerRuntime)
	err = pullAndRetagImage(sourceImage, imageName, containerRuntime)
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 成功拉取并重新标记镜像: %s\n", imageName)
}

// pullAndRetagImage 拉取镜像并重新标记
func pullAndRetagImage(sourceImage, targetImage, containerRuntime string) error {
	// 分割容器运行时命令，支持多词命令如 "k3s crictl"
	runtimeParts := strings.Fields(containerRuntime)
	if len(runtimeParts) == 0 {
		runtimeParts = []string{"docker"} // 默认使用docker
	}

	// 拉取源镜像
	fmt.Printf("执行: %s pull %s\n", containerRuntime, sourceImage)
	pullArgs := append(runtimeParts[1:], "pull", sourceImage)
	pullCmd := exec.Command(runtimeParts[0], pullArgs...)
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr

	if err := pullCmd.Run(); err != nil {
		return fmt.Errorf("拉取镜像失败: %w", err)
	}

	// 重新标记镜像
	fmt.Printf("执行: %s tag %s %s\n", containerRuntime, sourceImage, targetImage)
	tagArgs := append(runtimeParts[1:], "tag", sourceImage, targetImage)
	tagCmd := exec.Command(runtimeParts[0], tagArgs...)
	tagCmd.Stdout = os.Stdout
	tagCmd.Stderr = os.Stderr

	if err := tagCmd.Run(); err != nil {
		return fmt.Errorf("重新标记镜像失败: %w", err)
	}

	// 可选：删除源镜像以节省空间
	fmt.Printf("执行: %s rmi %s\n", containerRuntime, sourceImage)
	rmiArgs := append(runtimeParts[1:], "rmi", sourceImage)
	rmiCmd := exec.Command(runtimeParts[0], rmiArgs...)
	rmiCmd.Stdout = os.Stdout
	rmiCmd.Stderr = os.Stderr

	// 不强制删除，如果失败则忽略
	_ = rmiCmd.Run()

	return nil
}

// printUsage 打印使用说明
func printUsage() {
	fmt.Println("ImageShipper Pull - 镜像获取工具")
	fmt.Println("")
	fmt.Println("用法:")
	fmt.Println("  ./app pull <镜像名称> [选项]")
	fmt.Println("")
	fmt.Println("选项:")
	fmt.Println("  --podman      使用 Podman 而不是 Docker")
	fmt.Println("  --docker      使用 Docker（默认）")
	fmt.Println("  -e <命令>      使用自定义容器运行时命令，如 'k3s crictl'")
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  ./app pull nginx:latest")
	fmt.Println("  ./app pull nginx:latest --podman")
	fmt.Println("  ./app pull nginx:latest -e 'k3s crictl'")
	fmt.Println("  ./app pull custom/app:v1.0")
	fmt.Println("  ./app pull custom/app:v1.0 --podman")
	fmt.Println("  ./app pull custom/app:v1.0 -e 'k3s crictl'")
	fmt.Println("")
	fmt.Println("  并将其重新标记为指定的镜像名称（不添加前缀）")
}
