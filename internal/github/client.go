package github

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/go-github/v79/github"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"github.com/keevingness/image-shipper/internal/types"
)

// dispatchInterval 两次工作流触发的最小间隔，保证创建时间可区分，便于运行匹配
const dispatchInterval = 1100 * time.Millisecond

// Client GitHub客户端封装
type Client struct {
	client   *github.Client
	logger   *zap.Logger
	owner    string
	repo     string
	workflow string

	dispatchMu   sync.Mutex
	lastDispatch time.Time

	claimMu sync.Mutex
	// claimed 记录 requestID -> workflow run ID，保证轮询期间跟踪同一个运行
	claimed map[string]int64
}

// NewClient 创建新的GitHub客户端
func NewClient(token, owner, repo, workflow string, logger *zap.Logger) *Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	client := github.NewClient(tc)

	return &Client{
		client:   client,
		logger:   logger,
		owner:    owner,
		repo:     repo,
		workflow: workflow,
		claimed:  make(map[string]int64),
	}
}

// TriggerMirrorWorkflow 触发镜像转存工作流
func (c *Client) TriggerMirrorWorkflow(sourceImage, targetRegistry string) (*types.MirrorRequest, error) {
	// 串行化触发并保证最小间隔，避免并发 dispatch 落在同一秒无法区分
	c.dispatchMu.Lock()
	if !c.lastDispatch.IsZero() {
		if wait := dispatchInterval - time.Since(c.lastDispatch); wait > 0 {
			time.Sleep(wait)
		}
	}
	c.lastDispatch = time.Now()
	c.dispatchMu.Unlock()

	// 生成唯一ID（纳秒时间戳，支持并发触发）
	requestID := strconv.FormatInt(time.Now().UnixNano(), 10)

	// 准备工作流输入参数
	// 工作流文件期望接收一个名为docker_image的参数
	inputs := map[string]interface{}{
		"docker_image": sourceImage,
	}

	// 获取仓库默认分支，避免硬编码分支名
	repoInfo, _, err := c.client.Repositories.Get(context.Background(), c.owner, c.repo)
	if err != nil {
		c.logger.Error("Failed to get repo default branch", zap.Error(err))
		return nil, fmt.Errorf("failed to get repo default branch: %w", err)
	}
	ref := repoInfo.GetDefaultBranch()
	if ref == "" {
		ref = "main"
	}

	// 触发工作流
	event := github.CreateWorkflowDispatchEventRequest{
		Ref:    ref,
		Inputs: inputs,
	}

	_, err = c.client.Actions.CreateWorkflowDispatchEventByFileName(
		context.Background(),
		c.owner,
		c.repo,
		c.workflow,
		event,
	)
	if err != nil {
		c.logger.Error("Failed to trigger GitHub workflow", zap.Error(err))
		return nil, fmt.Errorf("failed to trigger workflow: %w", err)
	}

	// 创建请求记录
	request := &types.MirrorRequest{
		ID:             requestID,
		SourceImage:    sourceImage,
		TargetRegistry: targetRegistry,
		Status:         "pending",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	c.logger.Info("Successfully triggered mirror workflow",
		zap.String("request_id", requestID),
		zap.String("source_image", sourceImage),
		zap.String("target_registry", targetRegistry))

	return request, nil
}

// GetWorkflowStatus 获取工作流状态
// 首次查询时从触发时间之后的运行中认领一个（选创建时间最接近触发时间的），
// 后续轮询固定跟踪同一个运行，避免并发触发时互相干扰
func (c *Client) GetWorkflowStatus(requestID string) (*types.GitHubWorkflowResponse, error) {
	requestTime, err := parseRequestTime(requestID)
	if err != nil {
		return nil, err
	}

	// 已认领的运行直接查询
	c.claimMu.Lock()
	runID, claimed := c.claimed[requestID]
	c.claimMu.Unlock()
	if claimed {
		return c.getRunStatus(runID)
	}

	// 获取工作流运行列表，按创建时间降序排列
	runs, _, err := c.client.Actions.ListWorkflowRunsByFileName(
		context.Background(),
		c.owner,
		c.repo,
		c.workflow,
		&github.ListWorkflowRunsOptions{
			Event:       "workflow_dispatch",
			ListOptions: github.ListOptions{PerPage: 50},
		},
	)
	if err != nil {
		c.logger.Error("Failed to list workflow runs", zap.Error(err))
		return nil, fmt.Errorf("failed to list workflow runs: %w", err)
	}

	// 认领一个运行
	c.claimMu.Lock()
	selected := selectRun(runs.WorkflowRuns, requestTime, c.claimed)
	if selected != nil {
		c.claimed[requestID] = selected.GetID()
	}
	c.claimMu.Unlock()

	if selected == nil {
		// 触发的工作流尚未出现在运行列表中，视为 pending 继续等待
		return &types.GitHubWorkflowResponse{Status: "pending", Conclusion: "unknown"}, nil
	}

	return c.getRunStatus(selected.GetID())
}

// selectRun 在运行列表中为请求选择一个未认领的运行：
// 创建时间不早于触发时间前2秒，且与触发时间最接近
func selectRun(runs []*github.WorkflowRun, requestTime time.Time, claimed map[string]int64) *github.WorkflowRun {
	var best *github.WorkflowRun
	var bestDiff time.Duration
	for _, run := range runs {
		if run.CreatedAt == nil || run.CreatedAt.Before(requestTime.Add(-2*time.Second)) {
			continue
		}
		isClaimed := false
		for _, id := range claimed {
			if id == run.GetID() {
				isClaimed = true
				break
			}
		}
		if isClaimed {
			continue
		}
		diff := run.CreatedAt.Sub(requestTime)
		if diff < 0 {
			diff = -diff
		}
		if best == nil || diff < bestDiff {
			best, bestDiff = run, diff
		}
	}
	return best
}

// getRunStatus 查询单个工作流运行的详细状态
func (c *Client) getRunStatus(runID int64) (*types.GitHubWorkflowResponse, error) {
	runDetail, _, err := c.client.Actions.GetWorkflowRunByID(
		context.Background(),
		c.owner,
		c.repo,
		runID,
	)
	if err != nil {
		c.logger.Error("Failed to get workflow run details", zap.Error(err))
		return nil, fmt.Errorf("failed to get workflow run details: %w", err)
	}

	status := "unknown"
	if runDetail.Status != nil {
		status = *runDetail.Status
	}
	conclusion := "unknown"
	if runDetail.Conclusion != nil {
		conclusion = *runDetail.Conclusion
	}
	url := ""
	if runDetail.HTMLURL != nil {
		url = *runDetail.HTMLURL
	}

	return &types.GitHubWorkflowResponse{
		WorkflowID: runDetail.GetID(),
		Status:     status,
		Conclusion: conclusion,
		URL:        url,
	}, nil
}

// parseRequestTime 解析 requestID 中的触发时间，兼容纳秒/秒时间戳和 RFC3339 格式
func parseRequestTime(requestID string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, requestID); err == nil {
		return t, nil
	}
	if n, err := strconv.ParseInt(requestID, 10, 64); err == nil {
		if n > int64(time.Minute) { // 纳秒时间戳远大于秒时间戳
			return time.Unix(0, n), nil
		}
		return time.Unix(n, 0), nil
	}
	return time.Time{}, fmt.Errorf("invalid request_id format: %s", requestID)
}
