package model

import "time"

type ProjectStatus int8

const (
	ProjectStatusActive    ProjectStatus = 0
	ProjectStatusCompleted ProjectStatus = 1
)

type ProjectTaskStatus int8

const (
	ProjectTaskStatusPending         ProjectTaskStatus = 0
	ProjectTaskStatusPendingApproval ProjectTaskStatus = 1
	ProjectTaskStatusCompleted       ProjectTaskStatus = 2
)

type Project struct {
	ID          int               `json:"id" gorm:"primary_key"`
	Name        string            `json:"name" gorm:"column:name;not null"`
	Description *string           `json:"description" gorm:"column:description"`
	Color       *string           `json:"color" gorm:"column:color"`
	Icon        *string           `json:"icon" gorm:"column:icon"`
	CircleID    int               `json:"circleId" gorm:"column:circle_id;index;not null"`
	CreatedBy   int               `json:"createdBy" gorm:"column:created_by;not null"`
	DueDate     *time.Time        `json:"dueDate" gorm:"column:due_date"`
	Points      int               `json:"points" gorm:"column:points;default:0"`
	Status      ProjectStatus     `json:"status" gorm:"column:status;default:0"`
	IsDefault   bool              `json:"isDefault" gorm:"column:is_default;default:false"`
	CreatedAt   time.Time         `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   *time.Time        `json:"updatedAt,omitempty" gorm:"column:updated_at;autoUpdateTime"`
	Assignees   []ProjectAssignee `json:"assignees" gorm:"foreignkey:ProjectID;references:ID"`
	Tasks       []ProjectTask     `json:"tasks" gorm:"foreignkey:ProjectID;references:ID"`
}

// ProjectAssignee links a user to a project.
type ProjectAssignee struct {
	ID        int `json:"-" gorm:"primary_key"`
	ProjectID int `json:"projectId" gorm:"column:project_id;uniqueIndex:idx_proj_user"`
	UserID    int `json:"userId" gorm:"column:user_id;uniqueIndex:idx_proj_user"`
}

// ProjectTask is a single checklist item within a project.
type ProjectTask struct {
	ID          int               `json:"id" gorm:"primary_key"`
	ProjectID   int               `json:"projectId" gorm:"column:project_id;index;not null"`
	Name        string            `json:"name" gorm:"column:name;not null"`
	Description *string           `json:"description" gorm:"column:description"`
	Status      ProjectTaskStatus `json:"status" gorm:"column:status;default:0"`
	SortOrder   int               `json:"order" gorm:"column:sort_order;default:0"`
	CompletedBy *int              `json:"completedBy" gorm:"column:completed_by"`
	CompletedAt *time.Time        `json:"completedAt" gorm:"column:completed_at"`
	ApprovedBy  *int              `json:"approvedBy" gorm:"column:approved_by"`
	ApprovedAt  *time.Time        `json:"approvedAt" gorm:"column:approved_at"`
	CreatedAt   time.Time         `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   *time.Time        `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
}

// ProjectTaskReq is used in create/update requests. ID is nil for new tasks.
type ProjectTaskReq struct {
	ID          *int    `json:"id"`
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
	Order       int     `json:"order"`
}

type ProjectReq struct {
	Name        string           `json:"name" binding:"required"`
	Description *string          `json:"description"`
	Color       *string          `json:"color"`
	Icon        *string          `json:"icon"`
	DueDate     *string          `json:"dueDate"` // RFC3339 string from client
	Points      int              `json:"points"`
	AssigneeIDs []int            `json:"assigneeIds"`
	Tasks       []ProjectTaskReq `json:"tasks"`
}

type UpdateProjectReq struct {
	ID int `json:"id" binding:"required"`
	ProjectReq
}
