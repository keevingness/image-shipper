package docker

import "errors"

// Docker工具相关错误
var (
	// ErrInvalidImageRef 无效的镜像引用
	ErrInvalidImageRef = errors.New("invalid image reference")
)