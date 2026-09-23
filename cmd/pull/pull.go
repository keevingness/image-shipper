package pull

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

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
	// Run 执行命令并返回合并后的标准输出/标准错误
	Run(name string, args ...string) (string, error)
}

type execCommandRunner struct{}

func (execCommandRunner) Run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const spinnerInterval = 100 * time.Millisecond

type spinner struct {
	started time.Time
	stop    chan struct{}
	done    chan struct{}
	active  bool
}

// startSpinner 启动旋转动画；非终端环境（CI/测试）为空操作
func startSpinner(prefix string) *spinner {
	s := &spinner{
		started: time.Now(),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	if !isTerminal(os.Stdout) {
		close(s.done)
		return s
	}
	s.active = true
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(spinnerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-ticker.C:
				frame := spinnerFrames[int(time.Since(s.started)/spinnerInterval)%len(spinnerFrames)]
				fmt.Printf("\r\033[K%s %s (%ds)", frame, prefix, int(time.Since(s.started).Seconds()))
			}
		}
	}()
	return s
}

// Elapsed 返回自启动以来经过的时间
func (s *spinner) Elapsed() time.Duration { return time.Since(s.started) }

// Clear 停止动画并清除当前行
func (s *spinner) Clear() {
	if !s.active {
		return
	}
	close(s.stop)
	<-s.done
	fmt.Print("\r\033[K")
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// lastLines 返回多行文本的最后 n 行，空文本返回空串
func lastLines(s string, n int) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// Run 执行 pull 命令。
func Run() {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	filePath := fs.String("f", "", "指定Docker Compose/Kubernetes YAML或纯文本镜像列表文件路径")
	dryRunFlag := fs.Bool("dry-run", false, "仅显示拉取计划，不执行实际拉取操作")
	drFlag := fs.Bool("dr", false, "同 --dry-run")
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
	dryRun := *dryRunFlag || *drFlag

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
		for i := range images {
			images[i] = docker.TrimDockerHubPrefix(images[i])
		}
		fmt.Printf("从文件 %s 中解析出以下镜像:\n", *filePath)
		for i, image := range images {
			fmt.Printf("%d. %s\n", i+1, image)
		}

		successCount, errorCount := pullImages(images, *cfg, containerRuntime, dryRun, cfg.Concurrency, execCommandRunner{})

		if dryRun {
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
	if err := processImage(imageName, *cfg, containerRuntime, dryRun, execCommandRunner{}); err != nil {
		fmt.Printf("错误: %v\n", err)
		os.Exit(1)
	}
	if dryRun {
		fmt.Println("📝 注意: 运行在 dry-run 模式下，未执行实际拉取操作")
		return
	}
	fmt.Printf("✅ 成功拉取并重新标记镜像: %s\n", imageName)
}

func processImage(imageName string, cfg config.PullConfig, containerRuntime string, dryRun bool, runner commandRunner) error {
	return processImageWithLabel(imageName, cfg, containerRuntime, dryRun, "", false, runner)
}

// processImageWithLabel 处理单个镜像，并发模式下日志带序号前缀
func processImageWithLabel(imageName string, cfg config.PullConfig, containerRuntime string, dryRun bool, label string, withLabel bool, runner commandRunner) error {
	plan, err := buildPullPlan(imageName, cfg)
	if err != nil {
		return fmt.Errorf("无效的镜像地址 %q: %w", imageName, err)
	}
	if dryRun {
		if withLabel {
			fmt.Printf("[%s] 镜像: %s\n", label, imageName)
		}
		printPullPlan(plan)
		return nil
	}
	if withLabel {
		return executePullPlan(plan, label, containerRuntime, runner)
	}
	return executePullPlan(plan, "", containerRuntime, runner)
}

// pullImages 按并发数处理镜像列表，返回成功和失败数量
func pullImages(images []string, cfg config.PullConfig, containerRuntime string, dryRun bool, concurrency int, runner commandRunner) (successCount, errorCount int) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	taskCh := make(chan int)

	if concurrency > 1 {
		fmt.Printf("🚀 并发拉取 %d 个镜像，并发数: %d\n", len(images), concurrency)
	}

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range taskCh {
				image := images[i]
				label := fmt.Sprintf("[%d/%d]", i+1, len(images))
				if concurrency > 1 {
					fmt.Printf("▶ %s 开始拉取: %s\n", label, image)
				} else {
					fmt.Printf("\n正在处理镜像 %d/%d: %s\n", i+1, len(images), image)
				}
				if err := processImageWithLabel(image, cfg, containerRuntime, dryRun, label, concurrency > 1, runner); err != nil {
					fmt.Printf("❌ 处理镜像 %s 失败: %v\n", image, err)
					mu.Lock()
					errorCount++
					mu.Unlock()
					continue
				}
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	for i := range images {
		taskCh <- i
	}
	close(taskCh)
	wg.Wait()
	return successCount, errorCount
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

func executePullPlan(plan pullPlan, label string, containerRuntime string, runner commandRunner) error {
	runtimeParts := strings.Fields(containerRuntime)
	if len(runtimeParts) == 0 {
		runtimeParts = []string{"docker"}
	}
	tty := isTerminal(os.Stdout)

	step := func(i int) string {
		if label != "" {
			return fmt.Sprintf("%s [%d/%d]", label, i+1, len(plan.sources))
		}
		return fmt.Sprintf("[%d/%d]", i+1, len(plan.sources))
	}

	var pullErrors []error
	for i, source := range plan.sources {
		sp := startSpinner(fmt.Sprintf("%s 正在拉取 %s", step(i), source))
		if !tty {
			fmt.Printf("%s 执行: %s pull %s\n", step(i), containerRuntime, source)
		}
		pullArgs := append(append([]string{}, runtimeParts[1:]...), "pull", source)
		output, err := runner.Run(runtimeParts[0], pullArgs...)
		elapsed := sp.Elapsed().Round(time.Second)
		sp.Clear()
		if err != nil {
			fmt.Printf("❌ %s 拉取失败: %s (用时 %s)\n", step(i), source, elapsed)
			if tail := lastLines(output, 3); tail != "" {
				fmt.Printf("   %s\n", strings.ReplaceAll(tail, "\n", "\n   "))
			}
			pullErrors = append(pullErrors, fmt.Errorf("%s: %w", source, err))
			if i < len(plan.sources)-1 {
				fmt.Println("↪ 尝试下一个镜像站...")
			}
			continue
		}

		fmt.Printf("✅ %s 拉取成功 (用时 %s): %s\n", step(i), elapsed, source)
		if referencesEquivalent(source, plan.target) {
			return nil
		}

		fmt.Printf("🔄 %s 重新标记: %s -> %s\n", step(i), source, plan.target)
		tagArgs := append(append([]string{}, runtimeParts[1:]...), "tag", source, plan.target)
		if _, err := runner.Run(runtimeParts[0], tagArgs...); err != nil {
			return fmt.Errorf("重新标记镜像 %s 失败: %w", source, err)
		}

		rmiArgs := append(append([]string{}, runtimeParts[1:]...), "rmi", source)
		_, _ = runner.Run(runtimeParts[0], rmiArgs...)
		fmt.Printf("🧹 %s 已清理代理标签\n", step(i))
		return nil
	}

	return fmt.Errorf("共尝试 %d 个镜像地址，均拉取失败:\n%w", len(plan.sources), errors.Join(pullErrors...))
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
	fmt.Println("  ./app pull [选项] -f <docker-compose.yaml/k8s yaml/纯文本镜像列表文件路径>")
	fmt.Println("")
	fmt.Println("选项:")
	fmt.Println("  -f <文件路径>   指定Docker Compose/Kubernetes YAML或纯文本镜像列表文件路径")
	fmt.Println("  --dry-run, -dr  显示候选镜像地址，不执行实际拉取")
	fmt.Println("  --podman        使用Podman而不是Docker")
	fmt.Println("  --docker        使用Docker（默认）")
	fmt.Println("  -e <命令>       使用自定义容器运行时命令")
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  ./app pull postgres:15.19-trixie")
	fmt.Println("  ./app pull --dry-run postgres:15.19-trixie")
	fmt.Println("  ./app pull -dr postgres:15.19-trixie")
	fmt.Println("  ./app pull --podman nginx:latest")
	fmt.Println("  ./app pull -e 'k3s crictl' nginx:latest")
	fmt.Println("  ./app pull -f docker-compose.yaml")
	fmt.Println("  ./app pull -f images.txt")
	fmt.Println("")
	fmt.Println("Docker Hub 镜像默认从多个加速站依次拉取，成功后自动恢复原镜像名。")
	fmt.Println("设置 IMGSHIPPER_PULL_CONCURRENCY 可并发拉取多个镜像（默认 1，仅对 -f 文件模式生效）。")
}
