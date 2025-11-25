package yamlparser

import (
	"fmt"
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
	// 处理多文档YAML
	var images []string
	docs := splitYAML(content)

	// 至少需要包含有效的Kubernetes资源
	hasValidResource := false

	for _, doc := range docs {
		docImages, err := parseSingleK8sDoc(doc)
		if err != nil {
			// 对于单个文档解析失败，继续尝试其他文档
			fmt.Printf("警告: 解析单个文档失败: %v\n", err)
			continue
		}
		images = append(images, docImages...)
		// 只要有一个文档能被解析，就认为是有效的K8s文件
		hasValidResource = true
	}

	// 确保至少解析到了一个有效的Kubernetes资源
	if !hasValidResource {
		return images, fmt.Errorf("没有找到有效的Kubernetes资源")
	}

	return images, nil
}

// parseSingleK8sDoc 解析单个Kubernetes YAML文档并提取镜像
func parseSingleK8sDoc(doc string) ([]string, error) {
	// 使用通用map来解析YAML
	var data map[interface{}]interface{}
	if err := yaml.Unmarshal([]byte(doc), &data); err != nil {
		return nil, fmt.Errorf("解析YAML失败: %w", err)
	}

	// 检查是否是有效的Kubernetes资源
	_, hasAPIVersion := data["apiVersion"]
	_, hasKind := data["kind"]
	if !hasAPIVersion || !hasKind {
		return nil, fmt.Errorf("缺少必需的资源字段(apiVersion或kind)")
	}

	// 提取资源名称（如果有）
	// 可以移除对metadata和name的解析，因为我们不再需要输出调试日志

	// 尝试从spec中提取镜像
	var images []string
	if spec, ok := data["spec"].(map[interface{}]interface{}); ok {
		images = extractImagesFromSpec(spec)
	}

	// 即使没有找到镜像，也返回空切片而不是错误
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

// splitYAML 分割多文档YAML内容
func splitYAML(content string) []string {
	var docs []string
	parts := strings.Split(content, "---")

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			docs = append(docs, trimmed)
		}
	}

	return docs
}
