package config

import "testing"

func TestLoadPullWithDefaultsUsesMirrorsWithoutGitHubConfig(t *testing.T) {
	t.Setenv("IMGSHIPPER_GITHUB_TOKEN", "")
	t.Setenv("IMGSHIPPER_GITHUB_OWNER", "")
	t.Setenv("IMGSHIPPER_PULL_SOURCE_REGISTRY", "")

	cfg, err := LoadPullWithDefaults()
	if err != nil {
		t.Fatalf("LoadPullWithDefaults() error = %v", err)
	}
	if !cfg.UseDefaultMirrors {
		t.Fatal("UseDefaultMirrors = false, want true")
	}
	if cfg.ContainerRuntime != "docker" {
		t.Fatalf("ContainerRuntime = %q, want docker", cfg.ContainerRuntime)
	}
}

func TestLoadPullWithDefaultsPreservesConfiguredRegistry(t *testing.T) {
	t.Setenv("IMGSHIPPER_PULL_SOURCE_REGISTRY", "mirror.example.com/docker///")
	t.Setenv("IMGSHIPPER_PULL_CONTAINER_RUNTIME", "podman")

	cfg, err := LoadPullWithDefaults()
	if err != nil {
		t.Fatalf("LoadPullWithDefaults() error = %v", err)
	}
	if cfg.UseDefaultMirrors {
		t.Fatal("UseDefaultMirrors = true, want false")
	}
	if cfg.SourceRegistry != "mirror.example.com/docker" {
		t.Fatalf("SourceRegistry = %q", cfg.SourceRegistry)
	}
	if cfg.ContainerRuntime != "podman" {
		t.Fatalf("ContainerRuntime = %q, want podman", cfg.ContainerRuntime)
	}
}

func TestLoadPullWithDefaultsConcurrency(t *testing.T) {
	t.Setenv("IMGSHIPPER_PULL_CONCURRENCY", "")

	cfg, err := LoadPullWithDefaults()
	if err != nil {
		t.Fatalf("默认并发数加载失败: %v", err)
	}
	if cfg.Concurrency != 1 {
		t.Fatalf("默认 Concurrency = %d, 期望 1", cfg.Concurrency)
	}

	t.Setenv("IMGSHIPPER_PULL_CONCURRENCY", "4")
	cfg, err = LoadPullWithDefaults()
	if err != nil {
		t.Fatalf("并发数=4 加载失败: %v", err)
	}
	if cfg.Concurrency != 4 {
		t.Fatalf("Concurrency = %d, 期望 4", cfg.Concurrency)
	}

	for _, bad := range []string{"0", "-1", "abc"} {
		t.Setenv("IMGSHIPPER_PULL_CONCURRENCY", bad)
		if _, err := LoadPullWithDefaults(); err == nil {
			t.Errorf("Concurrency=%q 应当报错", bad)
		}
	}
}
