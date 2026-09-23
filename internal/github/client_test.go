package github

import (
	"strconv"
	"testing"
	"time"

	"github.com/google/go-github/v79/github"
)

func TestParseRequestTime(t *testing.T) {
	// 纳秒时间戳
	nanos := time.Now().UnixNano()
	got, err := parseRequestTime(strconv.FormatInt(nanos, 10))
	if err != nil {
		t.Fatalf("纳秒时间戳解析失败: %v", err)
	}
	if d := got.UnixNano() - nanos; d != 0 {
		t.Errorf("纳秒解析偏差: %d", d)
	}

	// 秒时间戳（旧格式兼容）
	secs := time.Now().Unix()
	got, err = parseRequestTime(strconv.FormatInt(secs, 10))
	if err != nil {
		t.Fatalf("秒时间戳解析失败: %v", err)
	}
	if got.Unix() != secs {
		t.Errorf("秒解析: got %d, want %d", got.Unix(), secs)
	}

	// 非法格式
	if _, err := parseRequestTime("not-a-timestamp"); err == nil {
		t.Error("非法格式应当报错")
	}
}

func TestSelectRunPicksClosestUnclaimed(t *testing.T) {
	now := time.Now()
	runAt := func(offset time.Duration, id int64) *github.WorkflowRun {
		ts := github.Timestamp{Time: now.Add(offset)}
		return &github.WorkflowRun{ID: &id, CreatedAt: &ts}
	}

	// 三个运行：分别在触发前 10s、触发后 1.1s、触发后 2.2s
	runs := []*github.WorkflowRun{
		runAt(-10*time.Second, 1),
		runAt(1100*time.Millisecond, 2),
		runAt(2200*time.Millisecond, 3),
	}

	// 未认领时应选时间最接近的 run 2
	got := selectRun(runs, now, map[string]int64{})
	if got == nil || got.GetID() != 2 {
		t.Fatalf("selectRun() = %v, 期望 run 2", got)
	}

	// run 2 被其他请求认领后应选 run 3
	claimed := map[string]int64{"req-a": 2}
	got = selectRun(runs, now, claimed)
	if got == nil || got.GetID() != 3 {
		t.Fatalf("selectRun() = %v, 期望 run 3", got)
	}

	// 全部被认领时返回 nil
	claimed["req-b"] = 3
	if got := selectRun(runs, now, claimed); got != nil {
		t.Fatalf("全部认领后应返回 nil, 得到 run %d", got.GetID())
	}

	// 触发时间 2 秒之前的运行不参与匹配
	old := []*github.WorkflowRun{runAt(-10*time.Second, 1)}
	if got := selectRun(old, now, map[string]int64{}); got != nil {
		t.Fatalf("过早的运行不应被选中, 得到 run %d", got.GetID())
	}
}
