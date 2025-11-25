package yamlparser

import (
	"fmt"
	"strings"
)

// FileType YAML文件类型
type FileType string

const (
	// FileTypeUnknown 未知文件类型
	FileTypeUnknown FileType = "unknown"
	// FileTypeCompose docker-compose文件
	FileTypeCompose FileType = "compose"
	// FileTypeK8s Kubernetes文件
	FileTypeK8s FileType = "k8s"
)

// ParseFile 解析YAML文件并提取镜像
// 根据文件名和内容综合判断是docker-compose还是k8s文件
func ParseFile(filePath string) ([]string, error) {
	// 首先根据文件名判断文件类型
	fileType := DetectFileType(filePath)
	
	// 根据检测到的文件类型优先使用对应解析器
	switch fileType {
	case FileTypeCompose:
		// 优先尝试作为docker-compose文件解析
		composeImages, err := ParseComposeFile(filePath)
		if err == nil {
			return composeImages, nil
		}
		// 如果失败，再尝试作为k8s文件解析
		return ParseK8sFile(filePath)
	case FileTypeK8s:
		// 优先尝试作为k8s文件解析
		k8sImages, err := ParseK8sFile(filePath)
		if err == nil {
			return k8sImages, nil
		}
		// 如果失败，再尝试作为docker-compose文件解析
		return ParseComposeFile(filePath)
	default:
		// 对于未知类型，按照原逻辑尝试
		// 首先尝试作为docker-compose文件解析
		composeImages, err := ParseComposeFile(filePath)
		if err == nil {
			return composeImages, nil
		}

		// 如果docker-compose解析失败，尝试作为k8s文件解析
		return ParseK8sFile(filePath)
	}
}

// ParseContent 解析YAML内容并提取镜像
// 根据内容自动判断是docker-compose还是k8s文件
func ParseContent(content string, fileType FileType) ([]string, error) {
	// 如果指定了文件类型，直接使用指定的解析器
	if fileType != FileTypeUnknown {
		switch fileType {
		case FileTypeCompose:
			return ParseComposeContent(content)
		case FileTypeK8s:
			return ParseK8sContent(content)
		default:
			return nil, fmt.Errorf("不支持的文件类型: %s", fileType)
		}
	}

	// 否则自动判断文件类型
	// 首先尝试作为docker-compose文件解析
	composeImages, err := ParseComposeContent(content)
	if err == nil {
		// 只要解析成功，即使没有找到镜像也认为是有效的compose文件
		return composeImages, nil
	}

	// 如果docker-compose解析失败，尝试作为k8s文件解析
	k8sImages, err := ParseK8sContent(content)
	if err == nil {
		// 只要解析成功，即使没有找到镜像也认为是有效的k8s文件
		return k8sImages, nil
	}

	// 如果两种解析都失败，返回详细错误
	return nil, fmt.Errorf("无法解析内容: 既不是有效的docker-compose内容，也不是有效的k8s内容")
}

// DetectFileType 根据文件路径或内容检测YAML文件类型
func DetectFileType(filePath string) FileType {
	// 简单根据文件名判断
	if strings.Contains(filePath, "docker-compose") || strings.Contains(filePath, "compose") {
		return FileTypeCompose
	}
	// 识别k8s相关文件名
	if strings.Contains(filePath, "k8s") || strings.Contains(filePath, "kubernetes") || strings.Contains(filePath, "istio") {
		return FileTypeK8s
	}
	// 其他可能的YAML文件扩展名
	if strings.HasSuffix(filePath, ".yaml") || strings.HasSuffix(filePath, ".yml") {
		// 暂时返回未知类型，让ParseFile函数自动尝试解析
		return FileTypeUnknown
	}
	return FileTypeUnknown
}
