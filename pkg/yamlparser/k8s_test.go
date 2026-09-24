package yamlparser

import "testing"

func TestParseK8sContentKeepsDocumentMarkerInBlockScalar(t *testing.T) {
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
spec:
  config: |
    ---
  template:
    spec:
      containers:
        - name: app
          image: nginx:latest
---
apiVersion: v1
kind: Service
metadata:
  name: demo
`

	images, err := ParseK8sContent(content)
	if err != nil {
		t.Fatalf("ParseK8sContent() error = %v", err)
	}
	if len(images) != 1 || images[0] != "nginx:latest" {
		t.Fatalf("ParseK8sContent() = %v, 期望 [nginx:latest]", images)
	}
}
