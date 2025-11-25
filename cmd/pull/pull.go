package pull

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/keevingness/image-shipper/internal/config"
	"github.com/keevingness/image-shipper/pkg/docker"
	"github.com/keevingness/image-shipper/pkg/yamlparser"
)

// Run 执行pull命令
func Run() {
	// 创建flag集合
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	filePath := fs.String("f", "", "指定Docker Compose或Kubernetes YAML文件路径")
	dryRun := fs.Bool("dry-run", false, "仅解析文件并显示镜像，不执行实际拉取操作")
	podmanFlag := fs.Bool("podman", false, "使用Podman而不是Docker")
	dockerFlag := fs.Bool("docker", false, "使用Docker（默认）")
	customRuntime := fs.String("e", "", "使用自定义容器运行时命令")

	// 解析参数
	if len(os.Args) <= 2 {
		printUsage()
		os.Exit(1)
	}

	// 检查是否请求帮助
	for _, arg := range os.Args[2:] {
		if arg == "--help" || arg == "-h" {
			printUsage()
			return
		}
	}

	fs.Parse(os.Args[2:])

	// 如果是文件模式且处于dry-run模式，不需要加载完整配置
	if *filePath != "" && *dryRun {
		// 直接解析文件并显示镜像
		images, err := yamlparser.ParseFile(*filePath)
		if err != nil {
			fmt.Printf("解析文件失败: %v\n", err)
			os.Exit(1)
		}

		// 显示解析出的镜像
		fmt.Printf("从文件 %s 中解析出以下镜像:\n", *filePath)
		for i, image := range images {
			fmt.Printf("%d. %s\n", i+1, image)
		}

		fmt.Println("\n📝 注意: 运行在dry-run模式下，未执行实际拉取操作")
		return
	}

	// 加载配置
	cfg, err := config.LoadWithDefaults()
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 确定容器运行时
	containerRuntime := cfg.Pull.ContainerRuntime
	if *podmanFlag {
		containerRuntime = "podman"
	} else if *dockerFlag {
		containerRuntime = "docker"
	} else if *customRuntime != "" {
		containerRuntime = *customRuntime
	}

	// 从配置中获取源镜像仓库地址
	sourceRegistry := cfg.Pull.SourceRegistry

	// 检查是否指定了文件路径
	if *filePath != "" {
		// 从文件中解析镜像
		images, err := yamlparser.ParseFile(*filePath)
		if err != nil {
			fmt.Printf("解析文件失败: %v\n", err)
			os.Exit(1)
		}

		// 显示解析出的镜像
		fmt.Printf("从文件 %s 中解析出以下镜像:\n", *filePath)
		for i, image := range images {
			fmt.Printf("%d. %s\n", i+1, image)
		}

		// 由于我们已经在前面处理了dry-run模式，这里不需要再检查

		// 处理每个镜像
		successCount := 0
		errorCount := 0

		for i, image := range images {
			fmt.Printf("\n正在处理镜像 %d/%d: %s\n", i+1, len(images), image)

			// 解析镜像地址
			_, _, tag, err := docker.ParseImageReference(image)
			if err != nil {
				fmt.Printf("❌ 跳过无效的镜像地址 %s: %v\n", image, err)
				errorCount++
				continue
			}

			// 如果没有指定标签，默认使用latest
			currentImage := image
			if tag == "" {
				currentImage = image + ":latest"
			}

			// 构建源镜像地址
			sourceImage := sourceRegistry + "/" + currentImage

			// 拉取镜像
			err = pullAndRetagImage(sourceImage, currentImage, containerRuntime)
			if err != nil {
				fmt.Printf("❌ 拉取镜像 %s 失败: %v\n", currentImage, err)
				errorCount++
			} else {
				fmt.Printf("✅ 成功拉取并重新标记镜像: %s\n", currentImage)
				successCount++
			}
		}

		// 打印总结
		fmt.Printf("\n📊 总结: 成功拉取 %d 个镜像，失败 %d 个镜像\n", successCount, errorCount)
		if errorCount > 0 {
			os.Exit(1)
		}
	} else {
		// 处理单个镜像
		if len(fs.Args()) == 0 {
			printUsage()
			os.Exit(1)
		}

		imageName := fs.Args()[0]
		if imageName == "" {
			fmt.Println("错误: 镜像名称不能为空")
			os.Exit(1)
		}

		// 如果是dry-run模式，只显示镜像信息
		if *dryRun {
			fmt.Printf("📝 注意: 运行在dry-run模式下，将从 %s 拉取镜像: %s\n", sourceRegistry, imageName)
			return
		}

		// 解析镜像地址
		_, _, tag, err := docker.ParseImageReference(imageName)
		if err != nil {
			fmt.Printf("错误: 无效的镜像地址格式: %v\n", err)
			os.Exit(1)
		}

		// 如果没有指定标签，默认使用latest
		if tag == "" {
			imageName = imageName + ":latest"
		}

		// 构建源镜像地址
		sourceImage := sourceRegistry + "/" + imageName

		// 拉取镜像
		fmt.Printf("正在从 %s 拉取镜像 %s (使用 %s)...\n", sourceRegistry, imageName, containerRuntime)
		err = pullAndRetagImage(sourceImage, imageName, containerRuntime)
		if err != nil {
			fmt.Printf("错误: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ 成功拉取并重新标记镜像: %s\n", imageName)
	}
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
	fmt.Println("  ./app pull -f <docker-compose.yaml或k8s yaml文件路径> [选项]")
	fmt.Println("  ./app pull -f <docker-compose.yaml或k8s yaml文件路径> --dry-run  # 仅解析文件并显示镜像，不执行实际拉取")
	fmt.Println("")
	fmt.Println("选项:")
	fmt.Println("  -f <文件路径>   指定Docker Compose或Kubernetes YAML文件路径")
	fmt.Println("  --dry-run       仅解析文件并显示镜像，不执行实际拉取操作")
	fmt.Println("  --podman        使用Podman而不是Docker")
	fmt.Println("  --docker        使用Docker（默认）")
	fmt.Println("  -e <命令>       使用自定义容器运行时命令，如 'k3s crictl'")
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  ./app pull nginx:latest")
	fmt.Println("  ./app pull nginx:latest --podman")
	fmt.Println("  ./app pull nginx:latest -e 'k3s crictl'")
	fmt.Println("  ./app pull -f docker-compose.yaml           # 从docker-compose文件中拉取所有镜像")
	fmt.Println("  ./app pull -f deployment.yaml              # 从Kubernetes deployment文件中拉取所有镜像")
	fmt.Println("  ./app pull -f docker-compose.yaml --dry-run  # 仅解析docker-compose文件中的镜像")
	fmt.Println("  ./app pull -f k8s-deployment.yaml --podman  # 使用Podman从K8s文件中拉取镜像")
	fmt.Println("")
	fmt.Println("  镜像将从配置的源仓库拉取，并重新标记为指定的镜像名称（不添加前缀）")
}
