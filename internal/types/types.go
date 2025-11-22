package types

import "time"

// MirrorRequest 镜像转存请求
type MirrorRequest struct {
	ID             string    `json:"id"`
	SourceImage    string    `json:"source_image"`
	TargetRegistry string    `json:"target_registry"`
	Status         string    `json:"status"` // pending, running, success, failed
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Error          string    `json:"error,omitempty"`
}

// GitHubWorkflowResponse GitHub工作流响应
type GitHubWorkflowResponse struct {
	WorkflowID int64  `json:"workflow_id"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	URL        string `json:"url"`
}
