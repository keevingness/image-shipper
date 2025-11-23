package main

import (
	"flag"
	"fmt"
	"github.com/keevingness/image-shipper/cmd"
)

// version变量，将在构建时通过-ldflags注入
var version = "dev"

func main() {
	// 添加版本标志
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "显示版本信息")
	flag.Parse()
	
	// 如果请求显示版本信息，则打印并退出
	if showVersion {
		fmt.Printf("image-shipper version %s\n", version)
		return
	}
	
	// 正常运行命令
	cmd.Run()
}
