package cmd

import (
	"fmt"
	"os"

	"github.com/keevingness/image-shipper/cmd/pull"
	"github.com/keevingness/image-shipper/cmd/ship"
)

func Run() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]
	switch command {
	case "ship":
		ship.Run()
	case "pull":
		pull.Run()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("未知命令: %s\n\n", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("ImageShipper")
	fmt.Println("")
	fmt.Println("用法:")
	fmt.Println("  ./app <command> [options]")
	fmt.Println("")
	fmt.Println("可用命令:")
	fmt.Println("  ship    转存 Docker 镜像")
	fmt.Println("  pull    获取并重新标记 Docker 镜像")
	fmt.Println("  help    显示帮助信息")
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  ./app ship nginx:latest  # 转存 nginx:latest 镜像")
	fmt.Println("  ./app pull nginx:latest  # 获取并重新标记 nginx:latest 镜像")
	fmt.Println("  ./app help      # 显示帮助信息")
}
