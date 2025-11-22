package ship

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/keevingness/image-shipper/internal/config"
	"github.com/keevingness/image-shipper/internal/github"
)

// Run 执行ship命令
func Run() {
	// 检查参数
	if len(os.Args) < 3 {
		printUsage()
		os.Exit(1)
	}

	imageURL := os.Args[2]
	if imageURL == "" {
		fmt.Println("错误: 镜像地址不能为空")
		os.Exit(1)
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

	// 触发工作流
	fmt.Printf("正在触发镜像转存工作流: %s\n", imageURL)
	request, err := githubClient.TriggerMirrorWorkflow(imageURL, "")
	if err != nil {
		fmt.Printf("触发工作流失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("工作流已触发，请求ID: %s\n", request.ID)
	fmt.Println("正在等待工作流执行完成...")

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

	// 初始状态显示
	fmt.Printf("\r工作流状态: %s %s, 结论: %s", spinners[0], currentStatus, currentConclusion)

	for {
		select {
		case <-spinnerTicker.C:
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
			currentStatus = response.Status
			currentConclusion = response.Conclusion

			// 检查工作流是否完成
			if response.Status == "completed" {
				// 清除当前行并显示最终结果
				fmt.Printf("\r")
				if response.Conclusion == "success" {
					fmt.Println("✅ 镜像转存成功!")
					fmt.Printf("工作流详情: %s\n", response.URL)
					os.Exit(0)
				} else {
					fmt.Printf("❌ 镜像转存失败: %s\n", response.Conclusion)
					fmt.Printf("工作流详情: %s\n", response.URL)
					os.Exit(1)
				}
			}

		case <-sigChan:
			fmt.Printf("\r")
			fmt.Println("收到中断信号，停止轮询")
			os.Exit(1)

		case <-timeout:
			fmt.Printf("\r")
			fmt.Println("⏰ 等待工作流完成超时")
			os.Exit(1)
		}
	}
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
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  ./app ship nginx:latest")
	fmt.Println("  ./app ship docker.io/library/nginx:latest")
	fmt.Println("")
	fmt.Println("环境变量:")
	fmt.Println("  GITHUB_TOKEN  GitHub访问令牌 (可选，也可在配置文件中设置)")
}
