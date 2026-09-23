package yamlparser

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ParseTextFile 解析纯文本文件并提取镜像
// 每行一个镜像地址，支持空行和以 # 开头的注释行
func ParseTextFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法打开文件: %w", err)
	}
	defer file.Close()

	var images []string
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.ContainsAny(line, " \t") {
			// 行内包含空白，取第一个字段作为镜像地址
			line = strings.Fields(line)[0]
		}
		images = append(images, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	return images, nil
}
