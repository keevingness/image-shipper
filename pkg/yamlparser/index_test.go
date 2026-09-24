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
