package yamlparser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFileDetectsKubernetesManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployment.yaml")
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
spec:
  template:
    spec:
      containers:
        - name: app
          image: nginx:latest
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	images, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if len(images) != 1 || images[0] != "nginx:latest" {
		t.Fatalf("ParseFile() = %v, 期望 [nginx:latest]", images)
	}
}

func TestParseFileDeduplicatesKubernetesImages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
spec:
  template:
    spec:
      initContainers:
        - name: setup
          image: busybox:latest
      containers:
        - name: app
          image: nginx:latest
        - name: sidecar
          image: nginx:latest
        - name: setup
          image: busybox
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	images, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	want := []string{"nginx:latest", "busybox", "busybox:latest"}
	if len(images) != len(want) {
		t.Fatalf("ParseFile() = %v, 期望 %v", images, want)
	}
	for i, image := range want {
		if images[i] != image {
			t.Fatalf("ParseFile() = %v, 期望 %v", images, want)
		}
	}
}

func TestParseContentAcceptsEmptyCompose(t *testing.T) {
	images, err := ParseContent("services: {}\n", FileTypeUnknown)
	if err != nil {
		t.Fatalf("ParseContent() error = %v", err)
	}
	if len(images) != 0 {
		t.Fatalf("ParseContent() = %v, 期望空结果", images)
	}
}

func TestParseContentFallsBackToKubernetes(t *testing.T) {
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
spec:
  template:
    spec:
      containers:
        - name: app
          image: redis:7
`

	images, err := ParseContent(content, FileTypeUnknown)
	if err != nil {
		t.Fatalf("ParseContent() error = %v", err)
	}
	if len(images) != 1 || images[0] != "redis:7" {
		t.Fatalf("ParseContent() = %v, 期望 [redis:7]", images)
	}
}
