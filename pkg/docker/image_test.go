package docker

import "testing"

func TestNormalizeImageReferenceForDockerHub(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		repository string
		target     string
		mirrorPath string
	}{
		{
			name:       "official image",
			input:      "postgres:15.19-trixie",
			repository: "library/postgres",
			target:     "postgres:15.19-trixie",
			mirrorPath: "postgres:15.19-trixie",
		},
		{
			name:       "official image with library namespace",
			input:      "library/postgres:15",
			repository: "library/postgres",
			target:     "postgres:15",
			mirrorPath: "postgres:15",
		},
		{
			name:       "official image with explicit registry",
			input:      "docker.io/library/postgres:15.19-trixie",
			repository: "library/postgres",
			target:     "postgres:15.19-trixie",
			mirrorPath: "postgres:15.19-trixie",
		},
		{
			name:       "official image without explicit library namespace",
			input:      "docker.io/postgres:15",
			repository: "library/postgres",
			target:     "postgres:15",
			mirrorPath: "postgres:15",
		},
		{
			name:       "namespace image",
			input:      "bitnami/postgresql:17",
			repository: "bitnami/postgresql",
			target:     "bitnami/postgresql:17",
			mirrorPath: "bitnami/postgresql:17",
		},
		{
			name:       "official image with index alias",
			input:      "index.docker.io/library/postgres:15",
			repository: "library/postgres",
			target:     "postgres:15",
			mirrorPath: "postgres:15",
		},
		{
			name:       "official image with registry alias",
			input:      "registry-1.docker.io/postgres:15",
			repository: "library/postgres",
			target:     "postgres:15",
			mirrorPath: "postgres:15",
		},
		{
			name:       "namespace image with explicit registry",
			input:      "docker.io/bitnami/postgresql:17",
			repository: "bitnami/postgresql",
			target:     "bitnami/postgresql:17",
			mirrorPath: "bitnami/postgresql:17",
		},
		{
			name:       "namespace image without tag",
			input:      "bitnami/postgresql",
			repository: "bitnami/postgresql",
			target:     "bitnami/postgresql:latest",
			mirrorPath: "bitnami/postgresql:latest",
		},
		{
			name:       "default tag",
			input:      "postgres",
			repository: "library/postgres",
			target:     "postgres:latest",
			mirrorPath: "postgres:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := NormalizeImageReference(tt.input)
			if err != nil {
				t.Fatalf("NormalizeImageReference() error = %v", err)
			}
			if !ref.DockerHub {
				t.Fatal("DockerHub = false, want true")
			}
			if ref.Repository != tt.repository {
				t.Errorf("Repository = %q, want %q", ref.Repository, tt.repository)
			}
			if got := ref.TargetReference(); got != tt.target {
				t.Errorf("TargetReference() = %q, want %q", got, tt.target)
			}
			if got := ref.DockerHubMirrorPath(); got != tt.mirrorPath {
				t.Errorf("DockerHubMirrorPath() = %q, want %q", got, tt.mirrorPath)
			}
		})
	}
}

func TestNormalizeImageReferenceForOtherRegistries(t *testing.T) {
	tests := []string{
		"quay.io/prometheus/prometheus:v3.0.0",
		"registry.local:5000/org/app:v1",
		"localhost:5000/app:v1",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			ref, err := NormalizeImageReference(input)
			if err != nil {
				t.Fatalf("NormalizeImageReference() error = %v", err)
			}
			if ref.DockerHub {
				t.Fatal("DockerHub = true, want false")
			}
			if got := ref.TargetReference(); got != input {
				t.Fatalf("TargetReference() = %q, want %q", got, input)
			}
		})
	}
}

func TestNormalizeImageReferenceRejectsUnsupportedReferences(t *testing.T) {
	for _, input := range []string{"", "postgres:", "docker.io/", "postgres@sha256:abcdef"} {
		t.Run(input, func(t *testing.T) {
			if _, err := NormalizeImageReference(input); err == nil {
				t.Fatalf("NormalizeImageReference(%q) error = nil", input)
			}
		})
	}
}
