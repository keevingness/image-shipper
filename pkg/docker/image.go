package docker

import (
	"strings"
)

// ImageReference 是规范化后的容器镜像引用。
type ImageReference struct {
	Registry   string
	Repository string
	Tag        string
	DockerHub  bool
}

// NormalizeImageReference 规范化带标签的 Docker 镜像引用。
// 当前不支持 digest 引用，因为拉取后无法安全地推断目标标签。
func NormalizeImageReference(input string) (ImageReference, error) {
	input = strings.TrimSpace(input)
	if input == "" || strings.ContainsAny(input, " \t\r\n") || strings.Contains(input, "@") {
		return ImageReference{}, ErrInvalidImageRef
	}

	name := input
	tag := "latest"
	lastSlash := strings.LastIndex(name, "/")
	lastColon := strings.LastIndex(name, ":")
	if lastColon > lastSlash {
		tag = name[lastColon+1:]
		name = name[:lastColon]
		if tag == "" {
			return ImageReference{}, ErrInvalidImageRef
		}
	}

	parts := strings.Split(name, "/")
	for _, part := range parts {
		if part == "" {
			return ImageReference{}, ErrInvalidImageRef
		}
	}

	registry := "docker.io"
	repository := name
	if len(parts) > 1 && isRegistry(parts[0]) {
		registry = parts[0]
		repository = strings.Join(parts[1:], "/")
	}

	dockerHub := isDockerHubRegistry(registry)
	if dockerHub {
		registry = "docker.io"
		if !strings.Contains(repository, "/") {
			repository = "library/" + repository
		}
	}

	if repository == "" {
		return ImageReference{}, ErrInvalidImageRef
	}

	return ImageReference{
		Registry:   registry,
		Repository: repository,
		Tag:        tag,
		DockerHub:  dockerHub,
	}, nil
}

// TargetReference 返回拉取成功后应保留在本地的镜像名。
func (r ImageReference) TargetReference() string {
	repository := r.Repository
	if r.DockerHub {
		repository = strings.TrimPrefix(repository, "library/")
		return repository + ":" + r.Tag
	}
	return r.Registry + "/" + repository + ":" + r.Tag
}

// DockerHubMirrorPath 返回 Docker Hub 镜像在代理站中的路径。
func (r ImageReference) DockerHubMirrorPath() string {
	if !r.DockerHub {
		return ""
	}
	return strings.TrimPrefix(r.Repository, "library/") + ":" + r.Tag
}

func isRegistry(part string) bool {
	return strings.Contains(part, ".") || strings.Contains(part, ":") || part == "localhost"
}

func isDockerHubRegistry(registry string) bool {
	switch registry {
	case "docker.io", "index.docker.io", "registry-1.docker.io":
		return true
	default:
		return false
	}
}

// TrimDockerHubPrefix 去掉显式的 Docker Hub registry 前缀（docker.io/ 等）。
// docker.io 是默认 registry，去掉后引用完全等价，
// 且可避免转存时目标仓库命名中出现 "docker.io" 层级。
// 支持可选的 "--platform=xxx" 前缀。
func TrimDockerHubPrefix(imageRef string) string {
	s := strings.TrimSpace(imageRef)
	if s == "" {
		return imageRef
	}
	platformPrefix := ""
	if strings.HasPrefix(s, "--platform") {
		idx := strings.IndexAny(s, " \t")
		if idx < 0 {
			return imageRef
		}
		platformPrefix = s[:idx]
		s = strings.TrimSpace(s[idx+1:])
	}
	for _, prefix := range []string{"docker.io/", "index.docker.io/", "registry-1.docker.io/"} {
		if strings.HasPrefix(s, prefix) {
			s = s[len(prefix):]
			break
		}
	}
	if platformPrefix == "" {
		return s
	}
	return platformPrefix + " " + s
}

// ParseImageReference 解析Docker镜像引用。
func ParseImageReference(imageRef string) (registry, image, tag string, err error) {
	ref, err := NormalizeImageReference(imageRef)
	if err != nil {
		return "", "", "", err
	}
	if ref.DockerHub {
		if hasExplicitRegistry(imageRef) {
			return ref.Registry, ref.Repository, ref.Tag, nil
		}
		return "", strings.TrimPrefix(ref.Repository, "library/"), ref.Tag, nil
	}
	return ref.Registry, ref.Repository, ref.Tag, nil
}

func hasExplicitRegistry(imageRef string) bool {
	name := strings.SplitN(imageRef, "@", 2)[0]
	first := strings.SplitN(name, "/", 2)[0]
	return strings.Contains(name, "/") && isRegistry(first)
}
