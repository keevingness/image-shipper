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
