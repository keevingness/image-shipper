package docker

import (
	"strings"
)

// ParseImageReference 解析Docker镜像引用
func ParseImageReference(imageRef string) (registry, image, tag string, err error) {
	// 默认值
	registry = ""
	image = ""
	tag = "latest" // 默认标签

	// 分割镜像引用
	parts := strings.Split(imageRef, ":")
	if len(parts) == 0 {
		return "", "", "", ErrInvalidImageRef
	}

	// 处理包含标签的情况
	if len(parts) > 1 {
		// 检查最后一部分是否是标签（通常较短且不包含/）
		potentialTag := parts[len(parts)-1]
		if !strings.Contains(potentialTag, "/") {
			tag = potentialTag
			imageRef = strings.Join(parts[:len(parts)-1], ":")
		}
	}

	// 分割仓库和镜像
	parts = strings.Split(imageRef, "/")
	if len(parts) == 0 {
		return "", "", "", ErrInvalidImageRef
	}

	// 如果包含域名或IP，则第一部分是仓库
	if len(parts) > 1 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") || parts[0] == "localhost") {
		registry = parts[0]
		image = strings.Join(parts[1:], "/")
	} else {
		image = strings.Join(parts, "/")
	}

	// 验证结果
	if image == "" {
		return "", "", "", ErrInvalidImageRef
	}

	return registry, image, tag, nil
}