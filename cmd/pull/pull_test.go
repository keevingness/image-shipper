package pull

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/keevingness/image-shipper/internal/config"
)

type commandCall struct {
	name string
	args []string
}

type fakeCommandRunner struct {
	calls   []commandCall
	results map[string]error
}

func (r *fakeCommandRunner) Run(name string, args ...string) (string, error) {
	r.calls = append(r.calls, commandCall{name: name, args: append([]string(nil), args...)})
	return "", r.results[strings.Join(append([]string{name}, args...), " ")]
}

func TestBuildPullPlanUsesDefaultDockerHubMirrors(t *testing.T) {
	plan, err := buildPullPlan("postgres:15.19-trixie", config.PullConfig{UseDefaultMirrors: true})
	if err != nil {
		t.Fatalf("buildPullPlan() error = %v", err)
	}

	want := pullPlan{
		target: "postgres:15.19-trixie",
		sources: []string{
			"swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/postgres:15.19-trixie",
			"gh-proxy.org/docker/postgres:15.19-trixie",
			"v4.gh-proxy.org/docker/postgres:15.19-trixie",
			"v6.gh-proxy.org/docker/postgres:15.19-trixie",
			"cdn.gh-proxy.org/docker/postgres:15.19-trixie",
		},
	}
	if !reflect.DeepEqual(plan, want) {
		t.Fatalf("buildPullPlan() = %#v, want %#v", plan, want)
	}
}

func TestExecutePullPlanFallsBackAndRetags(t *testing.T) {
	first := "mirror-1/docker/postgres:15"
	second := "mirror-2/docker/postgres:15"
	plan := pullPlan{target: "postgres:15", sources: []string{first, second}}
	runner := &fakeCommandRunner{results: map[string]error{"docker pull " + first: errors.New("unavailable")}}

	if err := executePullPlan(plan, "", "docker", runner); err != nil {
		t.Fatalf("executePullPlan() error = %v", err)
	}

	want := []commandCall{
		{name: "docker", args: []string{"pull", first}},
		{name: "docker", args: []string{"pull", second}},
		{name: "docker", args: []string{"tag", second, "postgres:15"}},
		{name: "docker", args: []string{"rmi", second}},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestExecutePullPlanDoesNotRetagEquivalentDockerHubReference(t *testing.T) {
	plan := pullPlan{
		target:  "postgres:15",
		sources: []string{"docker.io/library/postgres:15"},
	}
	runner := &fakeCommandRunner{results: map[string]error{}}

	if err := executePullPlan(plan, "", "docker", runner); err != nil {
		t.Fatalf("executePullPlan() error = %v", err)
	}

	want := []commandCall{
		{name: "docker", args: []string{"pull", "docker.io/library/postgres:15"}},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestExecutePullPlanReturnsErrorWhenAllMirrorsFail(t *testing.T) {
	first := "mirror-1/docker/postgres:15"
	second := "mirror-2/docker/postgres:15"
	plan := pullPlan{target: "postgres:15", sources: []string{first, second}}
	runner := &fakeCommandRunner{results: map[string]error{
		"docker pull " + first:  errors.New("first unavailable"),
		"docker pull " + second: errors.New("second unavailable"),
	}}

	err := executePullPlan(plan, "", "docker", runner)
	if err == nil {
		t.Fatal("executePullPlan() error = nil")
	}
	if !strings.Contains(err.Error(), first) || !strings.Contains(err.Error(), second) {
		t.Fatalf("error = %q, want both failed sources", err)
	}

	want := []commandCall{
		{name: "docker", args: []string{"pull", first}},
		{name: "docker", args: []string{"pull", second}},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestExecutePullPlanStopsWhenTagFails(t *testing.T) {
	first := "mirror-1/docker/postgres:15"
	second := "mirror-2/docker/postgres:15"
	plan := pullPlan{target: "postgres:15", sources: []string{first, second}}
	runner := &fakeCommandRunner{results: map[string]error{
		"docker tag " + first + " postgres:15": errors.New("tag failed"),
	}}

	if err := executePullPlan(plan, "", "docker", runner); err == nil {
		t.Fatal("executePullPlan() error = nil")
	}

	want := []commandCall{
		{name: "docker", args: []string{"pull", first}},
		{name: "docker", args: []string{"tag", first, "postgres:15"}},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestBuildPullPlanPullsOtherRegistriesDirectly(t *testing.T) {
	plan, err := buildPullPlan("quay.io/prometheus/prometheus:v3.0.0", config.PullConfig{UseDefaultMirrors: true})
	if err != nil {
		t.Fatalf("buildPullPlan() error = %v", err)
	}

	want := pullPlan{
		target:  "quay.io/prometheus/prometheus:v3.0.0",
		sources: []string{"quay.io/prometheus/prometheus:v3.0.0"},
	}
	if !reflect.DeepEqual(plan, want) {
		t.Fatalf("buildPullPlan() = %#v, want %#v", plan, want)
	}
}

func TestExecutePullPlanPullsOtherRegistryWithoutRetagging(t *testing.T) {
	image := "quay.io/prometheus/prometheus:v3.0.0"
	plan := pullPlan{target: image, sources: []string{image}}
	runner := &fakeCommandRunner{results: map[string]error{}}

	if err := executePullPlan(plan, "", "docker", runner); err != nil {
		t.Fatalf("executePullPlan() error = %v", err)
	}

	want := []commandCall{{name: "docker", args: []string{"pull", image}}}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestBuildPullPlanUsesConfiguredRegistry(t *testing.T) {
	plan, err := buildPullPlan("postgres:15", config.PullConfig{SourceRegistry: "mirror.example.com/docker"})
	if err != nil {
		t.Fatalf("buildPullPlan() error = %v", err)
	}

	want := pullPlan{
		target:  "postgres:15",
		sources: []string{"mirror.example.com/docker/postgres:15"},
	}
	if !reflect.DeepEqual(plan, want) {
		t.Fatalf("buildPullPlan() = %#v, want %#v", plan, want)
	}
}

func TestLastLines(t *testing.T) {
	if got := lastLines("", 3); got != "" {
		t.Errorf("lastLines(空串) = %q, 期望空串", got)
	}
	got := lastLines("l1\nl2\nl3\nl4\nl5\n", 2)
	want := "l4\nl5"
	if got != want {
		t.Errorf("lastLines = %q, 期望 %q", got, want)
	}
	if got := lastLines("a\nb", 5); got != "a\nb" {
		t.Errorf("lastLines = %q, 期望保留全部行", got)
	}
}

func TestPullImagesConcurrent(t *testing.T) {
	images := []string{"a:1", "b:1", "c:1", "d:1"}

	// runner 会阻塞直到同时有 expected 个任务在执行，以验证真实并发且不超过限制
	runner := newGatedRunner(2)

	successCount, errorCount := pullImages(images, config.PullConfig{Concurrency: 2}, "docker", false, 2, runner)
	if successCount != len(images) || errorCount != 0 {
		t.Fatalf("success=%d error=%d, want %d/0", successCount, errorCount, len(images))
	}
	if max := runner.maxInFlight(); max != 2 {
		t.Fatalf("最大并发 = %d, 期望正好 2", max)
	}
}

type gatedRunner struct {
	mu          sync.Mutex
	inFlight    int
	maxSeen     int
	expected    int
	reached     chan struct{}
	reachedOnce bool
}

func newGatedRunner(expected int) *gatedRunner {
	return &gatedRunner{expected: expected, reached: make(chan struct{})}
}

func (r *gatedRunner) maxInFlight() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.maxSeen
}

func (r *gatedRunner) Run(name string, args ...string) (string, error) {
	r.mu.Lock()
	r.inFlight++
	if r.inFlight > r.maxSeen {
		r.maxSeen = r.inFlight
	}
	if r.inFlight >= r.expected && !r.reachedOnce {
		r.reachedOnce = true
		close(r.reached)
	}
	r.mu.Unlock()

	<-r.reached // 等待并发度达到 expected 后才放行，保证确定性

	r.mu.Lock()
	r.inFlight--
	r.mu.Unlock()
	return "", nil
}
