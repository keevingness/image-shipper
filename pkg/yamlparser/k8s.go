package yamlparser

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v2"
)

// 简化的Kubernetes资源结构
type K8sResource struct {
	APIVersion string      `yaml:"apiVersion"`
	Kind       string      `yaml:"kind"`
	Metadata   interface{} `yaml:"metadata"`
	Spec       interface{} `yaml:"spec"`
}

// ParseK8sFile 解析Kubernetes YAML文件并提取所有镜像
func ParseK8sFile(filePath string) ([]string, error) {
	// 读取文件内容
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法读取文件 %s: %w", filePath, err)
	}

	return ParseK8sContent(string(data))
}

// ParseK8sContent 解析Kubernetes YAML内容并提取所有镜像
func ParseK8sContent(content string) ([]string, error) {
	decoder := yaml.NewDecoder(strings.NewReader(content))
	var images []string
	hasValidResource := false

	for {
		var data map[interface{}]interface{}
		if err := decoder.Decode(&data); err != nil {
			if err == io.EOF {
				break
			}
			return images, fmt.Errorf("解析YAML失败: %w", err)
		}
		if len(data) == 0 {
			continue
		}

		if _, hasAPIVersion := data["apiVersion"]; !hasAPIVersion {
			return images, fmt.Errorf("缺少必需的资源字段: apiVersion")
		}
		if _, hasKind := data["kind"]; !hasKind {
			return images, fmt.Errorf("缺少必需的资源字段: kind")
		}

		hasValidResource = true
		if spec, ok := data["spec"].(map[interface{}]interface{}); ok {
			images = append(images, extractImagesFromSpec(spec)...)
		}
	}

	if !hasValidResource {
		return images, fmt.Errorf("没有找到有效的Kubernetes资源")
	}

	return images, nil
}

// extractImagesFromSpec 从spec中提取镜像
func extractImagesFromSpec(spec map[interface{}]interface{}) []string {
	var images []string

	// 1. 直接从spec中提取容器镜像（Pod资源）
	if containers, ok := spec["containers"].([]interface{}); ok {
		images = append(images, extractImagesFromContainerList(containers)...)
	}

	// 提取initContainers镜像
	if initContainers, ok := spec["initContainers"].([]interface{}); ok {
		images = append(images, extractImagesFromContainerList(initContainers)...)
	}

	// 提取ephemeralContainers镜像
	if ephemeralContainers, ok := spec["ephemeralContainers"].([]interface{}); ok {
		images = append(images, extractImagesFromContainerList(ephemeralContainers)...)
	}

	// 2. 从template.spec中提取镜像（Deployment, StatefulSet等资源）
	if template, ok := spec["template"].(map[interface{}]interface{}); ok {
		if podSpec, ok := template["spec"].(map[interface{}]interface{}); ok {
			images = append(images, extractImagesFromSpec(podSpec)...)
		}
	}

	return images
}

// extractImagesFromContainerList 从容器列表中提取镜像
func extractImagesFromContainerList(containers []interface{}) []string {
	var images []string

	for _, container := range containers {
		if containerMap, ok := container.(map[interface{}]interface{}); ok {
			if image, ok := containerMap["image"].(string); ok && image != "" {
				images = append(images, image)
			}
		}
	}

	return images
}
