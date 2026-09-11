package pull

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/keevingness/image-shipper/internal/config"
	"github.com/keevingness/image-shipper/pkg/docker"
	"github.com/keevingness/image-shipper/pkg/yamlparser"
)

var defaultDockerHubMirrorTemplates = []string{
	"swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/%s",
	"gh-proxy.org/docker/%s",
	"v4.gh-proxy.org/docker/%s",
	"v6.gh-proxy.org/docker/%s",
	"cdn.gh-proxy.org/docker/%s",
}

type pullPlan struct {
	target  string
	sources []string
}

type commandRunner interface {
	Run(name string, args ...string) error
}

type execCommandRunner struct{}

func (execCommandRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Run 执行 pull 命令。
func Run() {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	filePath := fs.String("f", "", "指定Docker Compose或Kubernetes YAML文件路径")
	dryRun := fs.Bool("dry-run", false, "仅显示拉取计划，不执行实际拉取操作")
	podmanFlag := fs.Bool("podman", false, "使用Podman而不是Docker")
	dockerFlag := fs.Bool("docker", false, "使用Docker（默认）")
	customRuntime := fs.String("e", "", "使用自定义容器运行时命令")

	if len(os.Args) <= 2 {
		printUsage()
		os.Exit(1)
	}
	for _, arg := range os.Args[2:] {
		if arg == "--help" || arg == "-h" {
			printUsage()
			return
		}
	}
	fs.Parse(os.Args[2:])

	cfg, err := config.LoadPullWithDefaults()
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	containerRuntime := cfg.ContainerRuntime
	if *podmanFlag {
		containerRuntime = "podman"
	} else if *dockerFlag {
		containerRuntime = "docker"
	} else if *customRuntime != "" {
		containerRuntime = *customRuntime
	}

	if *filePath != "" {
		images, err := yamlparser.ParseFile(*filePath)
		if err != nil {
			fmt.Printf("解析文件失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("从文件 %s 中解析出以下镜像:\n", *filePath)
		for i, image := range images {
			fmt.Printf("%d. %s\n", i+1, image)
		}

		successCount := 0
		errorCount := 0
		for i, image := range images {
			fmt.Printf("\n正在处理镜像 %d/%d: %s\n", i+1, len(images), image)
			if err := processImage(image, *cfg, containerRuntime, *dryRun, execCommandRunner{}); err != nil {
				fmt.Printf("❌ 处理镜像 %s 失败: %v\n", image, err)
				errorCount++
				continue
			}
			successCount++
		}

		if *dryRun {
			fmt.Println("\n📝 注意: 运行在 dry-run 模式下，未执行实际拉取操作")
			return
		}
		fmt.Printf("\n📊 总结: 成功拉取 %d 个镜像，失败 %d 个镜像\n", successCount, errorCount)
		if errorCount > 0 {
			os.Exit(1)
		}
		return
	}

	if len(fs.Args()) == 0 || fs.Args()[0] == "" {
		printUsage()
		os.Exit(1)
	}

	imageName := fs.Args()[0]
	if err := processImage(imageName, *cfg, containerRuntime, *dryRun, execCommandRunner{}); err != nil {
		fmt.Printf("错误: %v\n", err)
		os.Exit(1)
	}
	if *dryRun {
		fmt.Println("📝 注意: 运行在 dry-run 模式下，未执行实际拉取操作")
		return
	}
	fmt.Printf("✅ 成功拉取并重新标记镜像: %s\n", imageName)
}

func processImage(imageName string, cfg config.PullConfig, containerRuntime string, dryRun bool, runner commandRunner) error {
	plan, err := buildPullPlan(imageName, cfg)
	if err != nil {
		return fmt.Errorf("无效的镜像地址 %q: %w", imageName, err)
	}
	if dryRun {
		printPullPlan(plan)
		return nil
	}
	return executePullPlan(plan, containerRuntime, runner)
}

func buildPullPlan(imageName string, cfg config.PullConfig) (pullPlan, error) {
	ref, err := docker.NormalizeImageReference(imageName)
	if err != nil {
		return pullPlan{}, err
	}

	plan := pullPlan{target: ref.TargetReference()}
	if cfg.SourceRegistry != "" {
		plan.sources = []string{strings.TrimSuffix(cfg.SourceRegistry, "/") + "/" + plan.target}
		return plan, nil
	}
	if ref.DockerHub && cfg.UseDefaultMirrors {
		mirrorPath := ref.DockerHubMirrorPath()
		for _, template := range defaultDockerHubMirrorTemplates {
			plan.sources = append(plan.sources, fmt.Sprintf(template, mirrorPath))
		}
		return plan, nil
	}

	plan.sources = []string{plan.target}
	return plan, nil
}

func executePullPlan(plan pullPlan, containerRuntime string, runner commandRunner) error {
	runtimeParts := strings.Fields(containerRuntime)
	if len(runtimeParts) == 0 {
		runtimeParts = []string{"docker"}
	}

	var pullErrors []error
	for i, source := range plan.sources {
		fmt.Printf("[%d/%d] 执行: %s pull %s\n", i+1, len(plan.sources), containerRuntime, source)
		pullArgs := append(append([]string{}, runtimeParts[1:]...), "pull", source)
		if err := runner.Run(runtimeParts[0], pullArgs...); err != nil {
			pullErrors = append(pullErrors, fmt.Errorf("%s: %w", source, err))
			if i < len(plan.sources)-1 {
				fmt.Println("拉取失败，尝试下一个镜像站...")
			}
			continue
		}

		if referencesEquivalent(source, plan.target) {
			return nil
		}

		fmt.Printf("执行: %s tag %s %s\n", containerRuntime, source, plan.target)
		tagArgs := append(append([]string{}, runtimeParts[1:]...), "tag", source, plan.target)
		if err := runner.Run(runtimeParts[0], tagArgs...); err != nil {
			return fmt.Errorf("重新标记镜像失败: %w", err)
		}

		fmt.Printf("执行: %s rmi %s\n", containerRuntime, source)
		rmiArgs := append(append([]string{}, runtimeParts[1:]...), "rmi", source)
		_ = runner.Run(runtimeParts[0], rmiArgs...)
		return nil
	}

	return fmt.Errorf("尝试了 %d 个镜像地址，均拉取失败: %w", len(plan.sources), errors.Join(pullErrors...))
}

func referencesEquivalent(source, target string) bool {
	sourceRef, sourceErr := docker.NormalizeImageReference(source)
	targetRef, targetErr := docker.NormalizeImageReference(target)
	if sourceErr != nil || targetErr != nil {
		return source == target
	}
	return sourceRef.Registry == targetRef.Registry &&
		sourceRef.Repository == targetRef.Repository &&
		sourceRef.Tag == targetRef.Tag
}

func printPullPlan(plan pullPlan) {
	fmt.Printf("目标镜像: %s\n", plan.target)
	fmt.Println("将按以下顺序尝试:")
	for i, source := range plan.sources {
		fmt.Printf("  %d. %s\n", i+1, source)
	}
}

func printUsage() {
	fmt.Println("ImageShipper Pull - 镜像获取工具")
	fmt.Println("")
	fmt.Println("用法:")
	fmt.Println("  ./app pull [选项] <镜像名称>")
	fmt.Println("  ./app pull [选项] -f <docker-compose.yaml或k8s yaml文件路径>")
	fmt.Println("")
	fmt.Println("选项:")
	fmt.Println("  -f <文件路径>   指定Docker Compose或Kubernetes YAML文件路径")
	fmt.Println("  --dry-run       显示候选镜像地址，不执行实际拉取")
	fmt.Println("  --podman        使用Podman而不是Docker")
	fmt.Println("  --docker        使用Docker（默认）")
	fmt.Println("  -e <命令>       使用自定义容器运行时命令")
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  ./app pull postgres:15.19-trixie")
	fmt.Println("  ./app pull --dry-run postgres:15.19-trixie")
	fmt.Println("  ./app pull --podman nginx:latest")
	fmt.Println("  ./app pull -e 'k3s crictl' nginx:latest")
	fmt.Println("  ./app pull -f docker-compose.yaml")
	fmt.Println("")
	fmt.Println("Docker Hub 镜像默认从多个加速站依次拉取，成功后自动恢复原镜像名。")
}
