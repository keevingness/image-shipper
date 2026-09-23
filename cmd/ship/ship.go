package ship

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/keevingness/image-shipper/internal/config"
	"github.com/keevingness/image-shipper/internal/github"
	"github.com/keevingness/image-shipper/pkg/docker"
	"github.com/keevingness/image-shipper/pkg/yamlparser"
)

// Run 执行ship命令
func Run() {
	// 解析命令行参数
	fs := flag.NewFlagSet("ship", flag.ExitOnError)
	filePath := fs.String("f", "", "指定Docker Compose/Kubernetes YAML或纯文本镜像列表文件路径")
	dryRunFlag := fs.Bool("dry-run", false, "仅解析文件并显示镜像，不执行实际推送操作")
	drFlag := fs.Bool("dr", false, "同 --dry-run")
	
	// 解析参数
	if len(os.Args) < 3 {
		printUsage()
		os.Exit(1)
	}
	
	// 解析标志
	fs.Parse(os.Args[2:])
	dryRun := *dryRunFlag || *drFlag
	
	// 检查是否指定了文件路径
	if *filePath != "" {
		// 从文件中解析镜像
		images, err := yamlparser.ParseFile(*filePath)
		if err != nil {
			fmt.Printf("解析文件失败: %v\n", err)
			os.Exit(1)
		}
		for i := range images {
			images[i] = docker.TrimDockerHubPrefix(images[i])
		}
		
		// 显示解析出的镜像
		fmt.Printf("从文件 %s 中解析出以下镜像:\n", *filePath)
		for i, image := range images {
			fmt.Printf("%d. %s\n", i+1, image)
		}
		
		// 如果是dry-run模式，则不执行实际推送
		if dryRun {
			fmt.Println("\n📝 注意: 运行在dry-run模式下，未执行实际推送操作")
			return
		}
		
		// 加载配置
		cfg, err := config.LoadWithDefaults()
		if err != nil {
			fmt.Printf("加载配置失败: %v\n", err)
			os.Exit(1)
		}

		// 初始化日志
		logger, err := initLogger()
		if err != nil {
			fmt.Printf("初始化日志失败: %v\n", err)
			os.Exit(1)
		}
		defer logger.Sync()

		// 创建GitHub客户端
		githubClient := github.NewClient(
			cfg.GitHub.Token,
			cfg.GitHub.Owner,
			cfg.GitHub.Repo,
			cfg.GitHub.Workflow,
			logger,
		)
		
		// 设置信号处理
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// 按并发数处理镜像
		shipImages(images, githubClient, logger, sigChan, cfg.Ship.Concurrency)
		return
	}
	
	// 如果没有指定文件，则使用传统方式处理单个镜像
	if len(fs.Args()) == 0 {
		printUsage()
		os.Exit(1)
	}
	
	imageURL := fs.Args()[0]
	if imageURL == "" {
		fmt.Println("错误: 镜像地址不能为空")
		os.Exit(1)
	}
	imageURL = docker.TrimDockerHubPrefix(imageURL)
	
	// 如果是dry-run模式，则不执行实际推送
	if dryRun {
		fmt.Printf("📝 注意: 运行在dry-run模式下，将处理镜像: %s\n", imageURL)
		return
	}
	
	// 加载配置
	cfg, err := config.LoadWithDefaults()
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	logger, err := initLogger()
	if err != nil {
		fmt.Printf("初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// 创建GitHub客户端
	githubClient := github.NewClient(
		cfg.GitHub.Token,
		cfg.GitHub.Owner,
		cfg.GitHub.Repo,
		cfg.GitHub.Workflow,
		logger,
	)
	
	// 设置信号处理，允许用户中断轮询
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// 触发单个镜像的工作流
	if err := shipSingleImage("", imageURL, githubClient, logger, sigChan, false); err != nil {
		fmt.Printf("错误: %v\n", err)
		os.Exit(1)
	}
	return
}

func shipImages(images []string, githubClient *github.Client, logger *zap.Logger, sigChan chan os.Signal, concurrency int) {
	if concurrency > len(images) {
		concurrency = len(images)
	}
	concurrent := concurrency > 1
	if concurrent {
		fmt.Printf("🚀 并发转存 %d 个镜像，并发数: %d\n", len(images), concurrency)
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	taskCh := make(chan int)
	successCount, errorCount := 0, 0

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range taskCh {
				image := images[i]
				label := fmt.Sprintf("[%d/%d]", i+1, len(images))
				if concurrent {
					fmt.Printf("▶ %s 开始转存: %s\n", label, image)
				} else {
					fmt.Printf("\n正在处理镜像 %d/%d: %s\n", i+1, len(images), image)
				}
				if err := shipSingleImage(label, image, githubClient, logger, sigChan, concurrent); err != nil {
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

	fmt.Printf("\n📊 总结: 成功转存 %d 个镜像，失败 %d 个镜像\n", successCount, errorCount)
	if errorCount > 0 {
		os.Exit(1)
	}
}

// shipSingleImage 处理单个镜像的转存
// label 为序号前缀（如 "[1/8]"），concurrent 为 true 时禁用单行动画并按行输出状态变化
func shipSingleImage(label string, imageURL string, githubClient *github.Client, logger *zap.Logger, sigChan chan os.Signal, concurrent bool) error {
	prefix := ""
	if label != "" {
		prefix = label + " "
	}
	// 触发工作流
	fmt.Printf("%s正在触发镜像转存工作流: %s\n", prefix, imageURL)
	request, err := githubClient.TriggerMirrorWorkflow(imageURL, "")
	if err != nil {
		return fmt.Errorf("触发工作流失败: %w", err)
	}

	if concurrent {
		fmt.Printf("%s工作流已触发 (ID: %s)，等待执行完成...\n", prefix, request.ID)
	} else {
		fmt.Printf("工作流已触发，请求ID: %s\n", request.ID)
		fmt.Println("正在等待工作流执行完成...")
	}

	// 轮询工作流状态
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// 快速更新进度指示器的定时器
	spinnerTicker := time.NewTicker(200 * time.Millisecond)
	defer spinnerTicker.Stop()

	timeout := time.After(30 * time.Minute) // 30分钟超时

	// 进度指示器字符
	spinners := []string{"|", "/", "-", "\\"}
	spinnerIndex := 0

	// 当前状态信息
	currentStatus := "in_progress"
	currentConclusion := "unknown"

	// 初始状态显示（仅顺序模式，并发模式下各行会交错，不使用单行动画）
	if !concurrent {
		fmt.Printf("\r工作流状态: %s %s, 结论: %s", spinners[0], currentStatus, currentConclusion)
	}

	for {
		select {
		case <-spinnerTicker.C:
			if concurrent {
				continue
			}
			// 更新进度指示器
			spinnerIndex = (spinnerIndex + 1) % len(spinners)
			fmt.Printf("\r工作流状态: %s %s, 结论: %s", spinners[spinnerIndex], currentStatus, currentConclusion)

		case <-ticker.C:
			// 检查工作流状态
			response, err := githubClient.GetWorkflowStatus(request.ID)
			if err != nil {
				logger.Error("获取工作流状态失败", zap.Error(err))
				currentStatus = "查询失败"
				currentConclusion = "未知"
				continue
			}

			// 更新状态信息
			if concurrent && (response.Status != currentStatus || response.Conclusion != currentConclusion) {
				fmt.Printf("ℹ️ %s%s 状态: %s\n", prefix, imageURL, response.Status)
			}
			currentStatus = response.Status
			currentConclusion = response.Conclusion

			// 检查工作流是否完成
			if response.Status == "completed" {
				if !concurrent {
					// 清除当前行
					fmt.Printf("\r")
				}
				if response.Conclusion == "success" {
					fmt.Printf("✅ %s镜像转存成功!\n", prefix)
					fmt.Printf("工作流详情: %s\n", response.URL)
					return nil
				}
				fmt.Printf("❌ %s镜像转存失败: %s\n", prefix, response.Conclusion)
				fmt.Printf("工作流详情: %s\n", response.URL)
				return fmt.Errorf("镜像转存失败: %s", response.Conclusion)
			}

		case <-sigChan:
			fmt.Printf("\r")
			fmt.Println("收到中断信号，停止轮询")
			os.Exit(1)

		case <-timeout:
			if !concurrent {
				fmt.Printf("\r")
			}
			return fmt.Errorf("等待工作流完成超时 (30分钟)")
		}
	}
}

// shipImagesFromFile 从YAML文件中解析镜像并转存
func shipImagesFromFile(filePath string, githubClient *github.Client, logger *zap.Logger) {
	fmt.Printf("正在解析文件: %s\n", filePath)
	
	// 解析YAML文件
	images, err := yamlparser.ParseFile(filePath)
	if err != nil {
		fmt.Printf("解析文件失败: %v\n", err)
		os.Exit(1)
	}
	
	if len(images) == 0 {
		fmt.Println("在文件中未找到任何镜像")
		os.Exit(0)
	}
	
	fmt.Printf("从文件中找到 %d 个镜像\n", len(images))
	for i, image := range images {
		fmt.Printf("%d. %s\n", i+1, image)
	}
	
	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 按顺序处理镜像
	shipImages(images, githubClient, logger, sigChan, 1)
}

// initLogger 初始化日志记录器
func initLogger() (*zap.Logger, error) {
	// 在生产环境中，可以使用更复杂的配置
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("创建日志记录器失败: %w", err)
	}

	return logger, nil
}

// printUsage 打印使用说明
func printUsage() {
	fmt.Println("ImageShipper Ship - 镜像转存工具")
	fmt.Println("")
	fmt.Println("用法:")
	fmt.Println("  ./app ship <镜像地址>")
	fmt.Println("  ./app ship -f <docker-compose.yaml/k8s yaml/纯文本镜像列表文件路径>")
	fmt.Println("  ./app ship -f <docker-compose.yaml/k8s yaml/纯文本镜像列表文件路径> --dry-run  # 仅解析文件并显示镜像，不执行实际推送")
	fmt.Println("")
	fmt.Println("选项:")
	fmt.Println("  -f <文件路径>   指定Docker Compose/Kubernetes YAML或纯文本镜像列表文件路径")
	fmt.Println("  --dry-run, -dr  仅解析文件并显示镜像，不执行实际推送操作")
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  ./app ship nginx:latest                     # 转存单个镜像")
	fmt.Println("  ./app ship docker.io/library/nginx:latest   # 转存单个镜像（完整路径）")
	fmt.Println("  ./app ship -f docker-compose.yaml           # 从docker-compose文件中转存所有镜像")
	fmt.Println("  ./app ship -f deployment.yaml              # 从Kubernetes deployment文件中转存所有镜像")
	fmt.Println("  ./app ship -f images.txt                   # 从纯文本镜像列表文件中转存所有镜像")
	fmt.Println("  ./app ship -f docker-compose.yaml --dry-run  # 仅解析docker-compose文件中的镜像")
	fmt.Println("  ./app ship -f images.txt -dr                # 同上，短参数写法")
	fmt.Println("")
	fmt.Println("环境变量:")
	fmt.Println("  GITHUB_TOKEN  GitHub访问令牌 (可选，也可在配置文件中设置)")
	fmt.Println("  IMGSHIPPER_SHIP_CONCURRENCY  并发转存镜像数（默认 1，仅对 -f 文件模式生效）")
}
