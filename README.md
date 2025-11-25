# Docker 镜像转存与代理服务

本项目实现一个 Docker 镜像转存与代理服务，主要包含三个核心功能：

1. 通过 GitHub Actions 将 Docker 镜像转存到指定远程仓库
2. 将指定镜像站的镜像拉取到本地并重新标记
3. 提供友好的命令行界面和灵活的配置管理
4. 支持多种容器运行时环境

## 功能特性

-   **镜像转存**：通过 GitHub Actions 自动将镜像从源仓库转存到目标仓库
-   **镜像拉取**：支持从指定镜像站拉取镜像并根据需要重新标记，实现镜像地址转换
-   **YAML 文件解析**：支持从 Docker Compose 和 Kubernetes YAML 文件中自动解析并提取所有镜像
-   **多容器运行时支持**：支持 Docker、Podman 和自定义容器运行时
-   **配置灵活**：支持环境变量和配置文件两种配置方式
-   **实时状态监控**：提供工作流执行状态的实时反馈
-   **文件类型智能识别**：根据文件名自动识别 YAML 文件类型，优先使用对应的解析器

## 安装

### Go 安装

```bash
go install github.com/keevingness/image-shipper@latest
```

### 从源码构建

```bash
git clone https://github.com/keevingness/image-shipper.git
cd image-shipper
go mod tidy
go build -o image-shipper main.go
```

### 二进制下载

从 [Releases](https://github.com/keevingness/image-shipper/releases) 页面下载适合您系统的二进制文件。

## 配置

### 配置文件位置

程序会按以下顺序查找配置文件：

1. 当前工作目录下的 `config.yaml`
2. `$HOME/.image-shipper/config.yaml`
3. `/etc/image-shipper/config.yaml`

### 配置优先级

配置优先级从高到低为：

1. 命令行参数
2. 环境变量
3. 配置文件

### 环境变量配置

可以通过以下环境变量进行配置：

```bash
# GitHub 相关配置
export IMGSHIPPER_GITHUB_TOKEN="your_github_token"
export IMGSHIPPER_GITHUB_OWNER="your_github_username"
export IMGSHIPPER_GITHUB_REPO="image-shipper"  # 默认值
export IMGSHIPPER_GITHUB_WORKFLOW="image-shipper.yaml"  # 默认值

# Pull 命令配置
export IMGSHIPPER_PULL_SOURCE_REGISTRY="docker.io/library"  # 默认值
export IMGSHIPPER_PULL_CONTAINER_RUNTIME="docker"  # 默认值
```

### 配置文件

创建 `config.yaml` 文件：

```yaml
github:
    token: "your_github_token"
    owner: "your_github_username"
    repo: "image-shipper"
    workflow: "image-shipper.yaml"

pull:
    source_registry: "docker.io/library"
    container_runtime: "docker"
```

## 使用方法

### 镜像转存 (ship 命令)

```bash
# 转存 nginx:latest 镜像
./image-shipper ship nginx:latest

# 转存指定仓库的镜像
./image-shipper ship docker.io/library/nginx:latest
```

### 镜像拉取 (pull 命令)

```bash
# 使用默认 Docker 拉取镜像
./image-shipper pull nginx:latest

# 使用 Podman 拉取镜像
./image-shipper pull nginx:latest --podman

# 使用自定义容器运行时
./image-shipper pull nginx:latest -e 'k3s crictl'

# 拉取自定义应用镜像
./image-shipper pull custom/app:v1.0

# 从 Docker Compose 文件中拉取所有镜像
./image-shipper pull -f docker-compose.yaml

# 从 Kubernetes YAML 文件中拉取所有镜像
./image-shipper pull -f kubernetes-manifest.yaml

# 仅显示文件中包含的镜像，不执行实际拉取
./image-shipper pull -f docker-compose.yaml --dry-run
./image-shipper pull -f kubernetes-manifest.yaml --dry-run
```

### 帮助信息

```bash
# 显示总体帮助
./image-shipper help

# 显示特定命令的帮助
./image-shipper ship --help
./image-shipper pull --help
```

## GitHub Actions 工作流

本项目包含一个 GitHub Actions 工作流文件 `.github/workflows/image-shipper.yaml`，用于实现镜像转存功能。

### 工作流配置

在 GitHub 仓库中设置以下 Secrets：

-   `ALIYUN_REGISTRY`: 阿里云镜像仓库地址
-   `ALIYUN_NAME_SPACE`: 阿里云命名空间
-   `ALIYUN_REGISTRY_USER`: 阿里云仓库用户名
-   `ALIYUN_REGISTRY_PASSWORD`: 阿里云仓库密码

### 工作流功能

1. 接收镜像地址作为输入参数
2. 从源仓库拉取镜像
3. 根据需要处理平台信息
4. 重新标记镜像并推送到目标仓库
5. 清理临时镜像以节省空间

## 项目结构

```
image-shipper/
├── .github/
│   └── workflows/
│       └── image-shipper.yaml    # GitHub Actions 工作流
├── cmd/
│   ├── pull/
│   │   └── pull.go               # Pull 命令实现
│   ├── root.go                   # 根命令和帮助信息
│   └── ship/
│       └── ship.go               # Ship 命令实现
├── internal/
│   ├── config/
│   │   └── config.go             # 配置管理
│   ├── github/
│   │   └── client.go             # GitHub API 客户端
│   └── types/
│       └── types.go              # 类型定义
├── pkg/
│   ├── docker/
│   │   ├── errors.go             # Docker 相关错误定义
│   │   └── image.go              # Docker 镜像处理工具
│   └── utils/
│       └── utils.go              # 通用工具函数
├── main.go                       # 程序入口
└── README.md                     # 项目文档
```

## 常见问题

### Q: 如何设置 GitHub Token？

A: 您可以在 GitHub 设置中创建 Personal Access Token，确保具有 `repo` 和 `workflow` 权限。

### Q: 支持哪些容器运行时？

A: 目前支持 Docker、Podman 以及任何兼容 Docker CLI 的自定义容器运行时。

### Q: 如何处理私有镜像？

A: 对于私有镜像，您需要确保 GitHub Actions 工作流环境有权限访问源镜像仓库，并在配置中提供必要的认证信息。

### Q: 工作流执行超时怎么办？

A: 默认超时时间为 30 分钟，您可以在 `cmd/ship/ship.go` 中修改 `timeout` 值。

## 项目引用

本项目中部分内容源自 [docker_image_pusher](https://github.com/tech-shrimp/docker_image_pusher) 项目。

## 许可证

本项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。

MIT 许可证允许您在保留版权声明和许可证声明的情况下，自由地使用、修改和分发本软件。
