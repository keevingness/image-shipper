package config

import (
	"fmt"
	"os"
)

// Config 应用程序配置结构
type Config struct {
	GitHub GitHubConfig `mapstructure:"github"`
	Pull   PullConfig   `mapstructure:"pull"`
}

// GitHubConfig GitHub相关配置
type GitHubConfig struct {
	Token    string `mapstructure:"token"`
	Owner    string `mapstructure:"owner"`
	Repo     string `mapstructure:"repo"`
	Workflow string `mapstructure:"workflow"`
}

// PullConfig Pull命令配置
type PullConfig struct {
	SourceRegistry   string `mapstructure:"source_registry"`
	ContainerRuntime string `mapstructure:"container_runtime"`
}

// LoadWithDefaults 从环境变量加载配置
func LoadWithDefaults() (*Config, error) {
	config := &Config{}

	// 直接从环境变量读取GitHub Token
	if token := os.Getenv("IMGSHIPPER_GITHUB_TOKEN"); token != "" {
		config.GitHub.Token = token
	}

	// 直接从环境变量读取其他GitHub配置
	if owner := os.Getenv("IMGSHIPPER_GITHUB_OWNER"); owner != "" {
		config.GitHub.Owner = owner
	}

	if repo := os.Getenv("IMGSHIPPER_GITHUB_REPO"); repo != "" {
		config.GitHub.Repo = repo
	}

	if workflow := os.Getenv("IMGSHIPPER_GITHUB_WORKFLOW"); workflow != "" {
		config.GitHub.Workflow = workflow
	}

	// 直接从环境变量读取Pull配置
	if sourceRegistry := os.Getenv("IMGSHIPPER_PULL_SOURCE_REGISTRY"); sourceRegistry != "" {
		config.Pull.SourceRegistry = sourceRegistry
	}

	if containerRuntime := os.Getenv("IMGSHIPPER_PULL_CONTAINER_RUNTIME"); containerRuntime != "" {
		config.Pull.ContainerRuntime = containerRuntime
	}

	// 设置默认值（只有在环境变量未设置时才应用）
	if config.GitHub.Repo == "" {
		config.GitHub.Repo = "image-shipper"
	}
	if config.GitHub.Workflow == "" {
		config.GitHub.Workflow = "image-shipper.yaml"
	}
	if config.Pull.SourceRegistry == "" {
		config.Pull.SourceRegistry = "docker.io/library"
	}
	if config.Pull.ContainerRuntime == "" {
		config.Pull.ContainerRuntime = "docker"
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return config, nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	// 在测试环境中，跳过GitHub配置验证
	if os.Getenv("TEST_ENV") != "true" {
		if c.GitHub.Token == "" {
			return fmt.Errorf("github token is required")
		}

		if c.GitHub.Owner == "" {
			return fmt.Errorf("github owner is required")
		}

		if c.GitHub.Repo == "" {
			return fmt.Errorf("github repo is required")
		}

		if c.GitHub.Workflow == "" {
			return fmt.Errorf("github workflow is required")
		}
	}
	return nil
}