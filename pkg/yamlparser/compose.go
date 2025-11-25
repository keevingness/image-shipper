package yamlparser

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// ComposeConfig docker-compose.yaml配置结构
type ComposeConfig struct {
	Version  string                 `yaml:"version"`
	Services map[string]ServiceConfig `yaml:"services"`
}

// ServiceConfig docker-compose服务配置
type ServiceConfig struct {
	Image string `yaml:"image"`
	Build string `yaml:"build"`
}

// ParseComposeFile 解析docker-compose.yaml文件并提取所有镜像
func ParseComposeFile(filePath string) ([]string, error) {
	// 读取文件内容
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法读取文件 %s: %w", filePath, err)
	}

	// 解析YAML
	var config ComposeConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("解析YAML文件失败: %w", err)
	}

	// 提取镜像
	images := []string{}
	for serviceName, service := range config.Services {
		// 如果服务有image字段，则提取镜像
		if service.Image != "" {
			images = append(images, service.Image)
		} else if service.Build != "" {
			// 如果服务是构建的，则忽略
			fmt.Printf("服务 %s 是构建的，跳过\n", serviceName)
		}
	}

	return images, nil
}

// ParseComposeContent 解析docker-compose.yaml内容并提取所有镜像
func ParseComposeContent(content string) ([]string, error) {
	// 解析YAML
	var config ComposeConfig
	err := yaml.Unmarshal([]byte(content), &config)
	if err != nil {
		return nil, fmt.Errorf("解析YAML内容失败: %w", err)
	}

	// 提取镜像
	images := []string{}
	for serviceName, service := range config.Services {
		// 如果服务有image字段，则提取镜像
		if service.Image != "" {
			images = append(images, service.Image)
		} else if service.Build != "" {
			// 如果服务是构建的，则忽略
			fmt.Printf("服务 %s 是构建的，跳过\n", serviceName)
		}
	}

	return images, nil
}
