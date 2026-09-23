package yamlparser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTextFile(t *testing.T) {
	content := `# 注释行
docker.io/rancher/klipper-helm:v0.13.3-build20260727

  docker.io/rancher/klipper-lb:v0.4.17
nginx:latest # 行尾注释应取第一个字段
`
	path := filepath.Join(t.TempDir(), "images.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	images, err := ParseTextFile(path)
	if err != nil {
		t.Fatalf("ParseTextFile() error = %v", err)
	}

	want := []string{
		"docker.io/rancher/klipper-helm:v0.13.3-build20260727",
		"docker.io/rancher/klipper-lb:v0.4.17",
		"nginx:latest",
	}
	if len(images) != len(want) {
		t.Fatalf("解析结果数量 = %d, 期望 %d: %v", len(images), len(want), images)
	}
	for i, w := range want {
		if images[i] != w {
			t.Errorf("images[%d] = %q, 期望 %q", i, images[i], w)
		}
	}
}

func TestParseTextFileEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(path, []byte("\n# 只有注释\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	images, err := ParseTextFile(path)
	if err != nil {
		t.Fatalf("ParseTextFile() error = %v", err)
	}
	if len(images) != 0 {
		t.Errorf("期望空结果, 得到 %v", images)
	}
}

func TestDetectFileTypeText(t *testing.T) {
	if got := DetectFileType("test.txt"); got != FileTypeText {
		t.Errorf("DetectFileType(test.txt) = %q, 期望 %q", got, FileTypeText)
	}
}

func TestParseFileWithTextFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(path, []byte("nginx:latest\nredis:7\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	images, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if len(images) != 2 || images[0] != "nginx:latest" || images[1] != "redis:7" {
		t.Errorf("ParseFile(test.txt) = %v, 期望 [nginx:latest redis:7]", images)
	}
}
