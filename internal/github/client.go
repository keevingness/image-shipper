package github

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v79/github"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"github.com/keevingness/image-shipper/internal/types"
)

// Client GitHub客户端封装
type Client struct {
	client   *github.Client
	logger   *zap.Logger
	owner    string
	repo     string
	workflow string
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
	}
}

// TriggerMirrorWorkflow 触发镜像转存工作流
func (c *Client) TriggerMirrorWorkflow(sourceImage, targetRegistry string) (*types.MirrorRequest, error) {
	// 生成唯一ID
	requestID := fmt.Sprintf("%d", time.Now().Unix())

	// 准备工作流输入参数
	// 工作流文件期望接收一个名为docker_image的参数
	inputs := map[string]interface{}{
		"docker_image": sourceImage,
	}

	// 触发工作流
	event := github.CreateWorkflowDispatchEventRequest{
		Ref:    "main",
		Inputs: inputs,
	}

	_, err := c.client.Actions.CreateWorkflowDispatchEventByFileName(
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
func (c *Client) GetWorkflowStatus(requestID string) (*types.GitHubWorkflowResponse, error) {
	// 获取工作流运行列表，按创建时间降序排列
	runs, _, err := c.client.Actions.ListWorkflowRunsByFileName(
		context.Background(),
		c.owner,
		c.repo,
		c.workflow,
		&github.ListWorkflowRunsOptions{
			Event: "workflow_dispatch",
		},
	)
	if err != nil {
		c.logger.Error("Failed to list workflow runs", zap.Error(err))
		return nil, fmt.Errorf("failed to list workflow runs: %w", err)
	}

	// 将request_id转换为时间戳，用于时间匹配
	requestTime, err := time.Parse(time.RFC3339, requestID)
	if err != nil {
		// 如果requestID不是RFC3339格式，尝试将其解析为Unix时间戳
		timestamp, err := time.Parse(time.RFC3339, time.Unix(0, 0).Add(time.Duration(parseInt(requestID))*time.Second).Format(time.RFC3339))
		if err != nil {
			return nil, fmt.Errorf("invalid request_id format: %w", err)
		}
		requestTime = timestamp
	}

	// 查找最近的工作流运行
	if len(runs.WorkflowRuns) > 0 {
		// 获取最新的工作流运行
		run := runs.WorkflowRuns[0]

		// 获取工作流运行的详细信息
		runDetail, _, err := c.client.Actions.GetWorkflowRunByID(
			context.Background(),
			c.owner,
			c.repo,
			run.GetID(),
		)
		if err != nil {
			c.logger.Error("Failed to get workflow run details", zap.Error(err))
			return nil, fmt.Errorf("failed to get workflow run details: %w", err)
		}

		// 检查工作流创建时间是否与request_id匹配（允许5分钟的时间差）
		if run.CreatedAt != nil {
			diff := run.CreatedAt.Sub(requestTime)
			if diff < 5*time.Minute && diff > -5*time.Minute {
				// 找到匹配的工作流运行
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
		}
	}

	return nil, fmt.Errorf("workflow run with request_id %s not found", requestID)
}

// parseInt 辅助函数，将字符串转换为int
func parseInt(s string) int64 {
	var result int64
	for _, r := range s {
		if r >= '0' && r <= '9' {
			result = result*10 + int64(r-'0')
		}
	}
	return result
}
